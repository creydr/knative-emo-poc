/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package eventmesh

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	eventmeshreconciler "knative.dev/eventmesh-operator/pkg/client/injection/reconciler/operator/v1alpha1/eventmesh"
	operatorv1alpha1listers "knative.dev/eventmesh-operator/pkg/client/listers/operator/v1alpha1"
	"knative.dev/eventmesh-operator/pkg/manifests"
	"knative.dev/eventmesh-operator/pkg/manifests/transform"
	"knative.dev/eventmesh-operator/pkg/reconciler/common"
	"knative.dev/eventmesh-operator/pkg/scaler"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
)

type Reconciler struct {
	eventMeshLister   operatorv1alpha1listers.EventMeshLister
	deploymentLister  appsv1listers.DeploymentLister
	scaler            *scaler.Scaler
	manifest          mf.Manifest
	crdLister         apiextensionsv1.CustomResourceDefinitionLister
	eventingParser    manifests.Parser
	kafkaBrokerParser manifests.Parser
}

// Check that our Reconciler implements eventmeshreconciler.Interface
var _ eventmeshreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, em *v1alpha1.EventMesh) reconciler.Event {
	precheckOk, err := r.runPrechecks(ctx, em)
	if err != nil {
		return fmt.Errorf("could not run prechecks: %w", err)
	}
	if !precheckOk {
		// prechecks failed and they set the status on their own
		return nil
	}

	stages := common.Stages{
		// TODO: run prechecks as part of this pipeline

		// load manifests
		manifests.AppendFromParser(ctx, r.eventingParser),
		manifests.AppendFromParser(ctx, r.kafkaBrokerParser),

		// apply scaling
		r.applyScaling,

		// add owner reference transformer
		r.addOwnerReference,

		// run transformers
		manifests.Transform,

		// install (delete + apply + post-install on upgrade)
		manifests.Install(r.manifest),

		// wait for ready
		r.checkDeployments,
	}

	err = stages.Execute(ctx, em)
	if err != nil {
		if common.IsNonRecoverableError(err) {
			// it doesn't make sense to retrigger a reconcile on non-recoverable errors
			return nil
		}
		return err
	}

	em.Status.MarkInstallSucceeded()

	return nil
}

func (r *Reconciler) hasForeignEventingInstalled(ctx context.Context, em *v1alpha1.EventMesh) (bool, error) {
	logger := logging.FromContext(ctx)

	d, err := r.deploymentLister.Deployments(system.Namespace()).Get("eventing-controller")
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to list deployments: %w", err)
	}

	if d.OwnerReferences != nil {
		eventMeshGVK := v1alpha1.SchemeGroupVersion.WithKind("EventMesh")
		for _, owner := range d.OwnerReferences {
			if owner.Kind == eventMeshGVK.Kind &&
				owner.APIVersion == eventMeshGVK.GroupVersion().String() &&
				owner.Name == em.GetName() {
				// the eventing-controller is owned by the EventMesh already

				return false, nil
			}
		}
	}

	logger.Warnf("Found eventing-controller deployment which got not installed from EventMesh operator: %v", d.ObjectMeta)
	return true, nil
}

func (r *Reconciler) applyScaling(ctx context.Context, manifests *manifests.Manifests, em *v1alpha1.EventMesh) error {
	if err := r.applyScalingForIMC(ctx, manifests, em); err != nil {
		return fmt.Errorf("failed to apply Scaling for IMC: %w", err)
	}

	if err := r.applyScalingForMTBroker(ctx, manifests, em); err != nil {
		return fmt.Errorf("failed to apply Scaling for MT broker: %w", err)
	}

	return nil
}

func (r *Reconciler) applyScalingForIMC(ctx context.Context, manifests *manifests.Manifests, em *v1alpha1.EventMesh) error {
	logger := logging.FromContext(ctx).With("scaler", "imc")

	imcScaleTarget, err := r.scaler.IMCScaleTarget()
	if err != nil {
		return fmt.Errorf("failed to get scale target for IMC components: %w", err)
	}

	logger.Debugf("Scaling in-memory channel components to %d", imcScaleTarget)

	// scale imc deployments directly as they are not managed via HPA
	addDeploymentScaleTransformerIfNeeded("imc-dispatcher", system.Namespace(), imcScaleTarget, manifests, em, logger)
	addDeploymentScaleTransformerIfNeeded("imc-controller", system.Namespace(), imcScaleTarget, manifests, em, logger)

	return nil
}

func (r *Reconciler) applyScalingForMTBroker(ctx context.Context, manifests *manifests.Manifests, em *v1alpha1.EventMesh) error {
	logger := logging.FromContext(ctx).With("scaler", "mt-broker")

	mtBrokerScaleTarget, err := r.scaler.MTBrokerScaleTarget()
	if err != nil {
		return fmt.Errorf("failed to get scale target for MT Broker components: %w", err)
	}

	logger.Debugf("Scaling mt broker components to %d", mtBrokerScaleTarget)

	addHPATransformerIfNeeded("broker-filter-hpa", "mt-broker-filter", system.Namespace(), mtBrokerScaleTarget, manifests, em, logger)
	addHPATransformerIfNeeded("broker-ingress-hpa", "mt-ingress-filter", system.Namespace(), mtBrokerScaleTarget, manifests, em, logger)
	// mt-broker controller is not managed via HPA therefor scale deployment directly
	addDeploymentScaleTransformerIfNeeded("mt-broker-controller", system.Namespace(), mtBrokerScaleTarget, manifests, em, logger)

	return nil
}

func addHPATransformerIfNeeded(hpaName, deploymentName, namespace string, scaleTarget int64, manifests *manifests.Manifests, em *v1alpha1.EventMesh, logger *zap.SugaredLogger) {
	if !em.Spec.Overrides.Workloads.GetByName(deploymentName).HasAtLeastOneWithReplicasSet() {

		if scaleTarget == 0 {
			// we can't scale to 0 with HPA unless HPAScaleToZero feature gate is enabled. --> remove HPA and scale deployment instead
			logger.Debugf("minReplicas for %s is set to 0. This can't be set to 0 unless HPAScaleToZero feature gate is enabled. Therefor removing HPA and scaling deployment instead", hpaName)

			// add HPA to list of elements which should be deleted and remove from toApply list
			hpaFilter := mf.All(mf.ByKind("HorizontalPodAutoscaler"), mf.ByName(hpaName))
			manifests.AddToDelete(manifests.ToApply.Filter(hpaFilter))
			manifests.FilterToApply(mf.Not(hpaFilter))

			// scale deployment instead of using HPA
			manifests.AddTransformers(
				transform.Scale(schema.FromAPIVersionAndKind("apps/v1", "Deployment"),
					deploymentName,
					namespace,
					scaleTarget))
		} else {
			manifests.AddTransformers(
				transform.HPAReplicas(
					hpaName,
					namespace,
					scaleTarget))
		}
	} else {
		logger.Debugf("Skipping to adjust %s HPA to %d replicas, as a workload override for its deployment (%s) exists which sets the replicas already", hpaName, scaleTarget, deploymentName)
	}
}

func addDeploymentScaleTransformerIfNeeded(deploymentName, namespace string, scaleTarget int64, manifests *manifests.Manifests, em *v1alpha1.EventMesh, logger *zap.SugaredLogger) {
	if !em.Spec.Overrides.Workloads.GetByName(deploymentName).HasAtLeastOneWithReplicasSet() {
		manifests.AddTransformers(
			transform.Scale(schema.FromAPIVersionAndKind("apps/v1", "Deployment"),
				deploymentName,
				namespace,
				scaleTarget))
	} else {
		logger.Debugf("Skipping to scale %s deployment to %d, as a workload override for this deployment exists which sets the replicas already", deploymentName, scaleTarget)
	}
}

// runPrechecks runs some prechecks before applying the manifests.
// returns true if all checks were passed or false if at least one check failed
func (r *Reconciler) runPrechecks(ctx context.Context, em *v1alpha1.EventMesh) (bool, error) {
	// check if eventing is installed already and not owned by EM
	alreadyInstalled, err := r.hasForeignEventingInstalled(ctx, em)
	if err != nil {
		return false, fmt.Errorf("could not determine whether EventMesh is installed already: %v", err)
	}
	if alreadyInstalled {
		em.Status.MarkInstallFailed("EventingInstalledAlready", "Knative eventing components seem to be installed already and not owned by the EventMesh")
		return false, nil
	}

	// check if transport-encryption is enabled and required cert-manager is installed
	featuresFlags, err := em.Spec.Features.GetEventingFeatureFlags()
	if err != nil {
		return false, fmt.Errorf("could not get feature flags: %v", err)
	}
	if !featuresFlags.IsDisabledTransportEncryption() {
		// TLS is enabled, we need cert-manager
		certManagerInstalled, err := r.hasCertManagerInstalled()
		if err != nil {
			return false, fmt.Errorf("could not determine whether cert-manager is installed: %v", err)
		}
		if !certManagerInstalled {
			em.Status.MarkInstallFailed("CertManagerRequired", "Feature %s is enabled, but cert-manager seems not to be installed", feature.TransportEncryption)
			return false, nil
		}
	}

	// all checks passed
	return true, nil
}

func (r *Reconciler) hasCertManagerInstalled() (bool, error) {
	// TODO: migrate usages to use util.IsCertmanagerInstalled() instead

	_, err := r.crdLister.Get("certificates.cert-manager.io")
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get certificates.cert-manager.io CRD: %w", err)
	}

	return true, nil
}

func (r *Reconciler) addOwnerReference(ctx context.Context, manifests *manifests.Manifests, em *v1alpha1.EventMesh) error {
	// TODO: fix (somehow the GVK of the em are empty)
	emCopy := em.DeepCopy()
	emCopy.SetGroupVersionKind(v1alpha1.SchemeGroupVersion.WithKind("EventMesh"))
	manifests.AddTransformers(transform.InjectOwner(emCopy)) // use our own InjectOwners to keep the namespace clean

	return nil
}

func (r *Reconciler) checkDeployments(ctx context.Context, manifests *manifests.Manifests, em *v1alpha1.EventMesh) error {
	var nonReadyDeployments []string
	for _, u := range manifests.ToApply.Filter(mf.ByKind("Deployment")).Resources() {
		deployment, err := r.deploymentLister.Deployments(u.GetNamespace()).Get(u.GetName())
		if err != nil {
			em.Status.MarkDeploymentsNotReady([]string{"all"})
			if apierrors.IsNotFound(err) {
				return nil
			}

			return fmt.Errorf("failed to get deployment %s/%s: %w", u.GetNamespace(), u.GetName(), err)
		}

		if !isDeploymentAvailable(deployment) {
			nonReadyDeployments = append(nonReadyDeployments, deployment.Name)
		}
	}

	if len(nonReadyDeployments) > 0 {
		em.Status.MarkDeploymentsNotReady(nonReadyDeployments)
		return common.DeploymentsNotReadyError{}
	}

	em.Status.MarkDeploymentsAvailable()
	return nil
}

func isDeploymentAvailable(d *appsv1.Deployment) bool {
	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
