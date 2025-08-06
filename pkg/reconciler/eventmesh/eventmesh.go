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
	"strings"
	"sync/atomic"

	"github.com/creydr/knative-emo-poc/pkg/apis/operator/v1alpha1"
	eventmeshreconciler "github.com/creydr/knative-emo-poc/pkg/client/injection/reconciler/operator/v1alpha1/eventmesh"
	operatorv1alpha1listers "github.com/creydr/knative-emo-poc/pkg/client/listers/operator/v1alpha1"
	knmf "github.com/creydr/knative-emo-poc/pkg/manifests"
	"github.com/creydr/knative-emo-poc/pkg/manifests/transform"
	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	messagingv1listers "knative.dev/eventing/pkg/client/listers/messaging/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/system"
)

type Reconciler struct {
	eventMeshLister       operatorv1alpha1listers.EventMeshLister
	manifest              mf.Manifest
	inMemoryChannelLister messagingv1listers.InMemoryChannelLister
	imcLister             *atomic.Pointer[messagingv1listers.InMemoryChannelLister]
	deploymentLister      appsv1listers.DeploymentLister
}

// Check that our Reconciler implements eventmeshreconciler.Interface
var _ eventmeshreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, em *v1alpha1.EventMesh) reconciler.Event {
	logger := logging.FromContext(ctx)

	alreadyInstalled, err := r.hasForeignEventingInstalled(ctx, em)
	if err != nil {
		return fmt.Errorf("could not determine whether EventMesh is installed already: %v", err)
	}
	if alreadyInstalled {
		em.Status.MarkEventMeshConditionEventMeshInstalledFalse("EventingInstalledAlready", "Knative eventing components seem to be installed already and not owned by the EventMesh")
		return nil
	}

	manifests := knmf.Manifests{}

	// Get eventing manifests
	logger.Debug("Loading eventing core manifests")
	eventingManifests, err := knmf.ForEventing(em)
	if err != nil {
		return fmt.Errorf("failed to get eventing manifests: %w", err)
	}
	manifests.Append(eventingManifests)

	// Get EKB manifests
	logger.Debug("Loading eventing-kafka-broker manifests")
	ekbManifests, err := knmf.ForEventingKafkaBroker(em)
	if err != nil {
		return fmt.Errorf("failed to get EKB manifests: %w", err)
	}
	manifests.Append(ekbManifests)

	if err := r.applyScaling(&manifests); err != nil {
		return fmt.Errorf("failed to apply Scaling: %w", err)
	}

	// Add owner reference to EventMesh CR
	// TODO: fix (somehow the GVK of the em are empty)
	emCopy := em.DeepCopy()
	emCopy.SetGroupVersionKind(v1alpha1.SchemeGroupVersion.WithKind("EventMesh"))
	manifests.AddTransformers(mf.InjectOwner(emCopy))

	// Apply patches at the end when all manifests are loaded
	logger.Debug("Applying patches to manifests")
	if err := manifests.TransformToApply(); err != nil {
		return fmt.Errorf("failed to transform eventing manifests to apply: %w", err)
	}
	// also run the transformers on the manifests which gets deleted, in case some metadata were patched before they were applied
	if err := manifests.TransformToDelete(); err != nil {
		return fmt.Errorf("failed to transform eventing manifests to delete: %w", err)
	}

	// Delete old manifests
	logger.Debug("Deleting unneeded manifests")
	if err := r.manifest.Append(manifests.ToDelete).Delete(mf.IgnoreNotFound(true)); err != nil {
		if !meta.IsNoMatchError(err) && !strings.Contains(err.Error(), "failed to get API group resources") {
			return fmt.Errorf("failed to delete manifests: %w", err)
		}
	}

	// Apply manifests
	logger.Debug("Applying manifests")
	if err := r.manifest.Append(manifests.ToApply).Apply(); err != nil {
		return fmt.Errorf("failed to apply manifests: %w", err)
	}

	em.Status.MarkEventMeshConditionEventMeshInstalled()

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

func (r *Reconciler) applyScaling(manifests *knmf.Manifests) error {
	var scaleTarget int

	// TODO: add mutex check
	imcLister := r.imcLister.Load()
	//if r.inMemoryChannelLister == nil {
	if imcLister == nil || *imcLister == nil {
		// no imc lister registered so far (probably because no IMC CRD is installed yet)
		scaleTarget = 0
	} else {
		//imcs, err := r.inMemoryChannelLister.List(labels.Everything())
		imcs, err := (*imcLister).List(labels.Everything())
		if err != nil {
			return fmt.Errorf("failed to list all InMemoryChannels: %w", err)
		}

		if len(imcs) == 0 {
			scaleTarget = 0
		} else {
			scaleTarget = 1
		}
	}

	manifests.AddTransformers(
		transform.Scale(schema.FromAPIVersionAndKind("apps/v1", "Deployment"),
			"imc-dispatcher",
			"knative-eventing",
			scaleTarget))
	manifests.AddTransformers(
		transform.Scale(schema.FromAPIVersionAndKind("apps/v1", "Deployment"),
			"imc-controller",
			"knative-eventing",
			scaleTarget))

	return nil
}
