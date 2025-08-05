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

	"github.com/creydr/knative-emo-poc/pkg/apis/operator/v1alpha1"
	eventmeshreconciler "github.com/creydr/knative-emo-poc/pkg/client/injection/reconciler/operator/v1alpha1/eventmesh"
	operatorv1alpha1listers "github.com/creydr/knative-emo-poc/pkg/client/listers/operator/v1alpha1"
	knmf "github.com/creydr/knative-emo-poc/pkg/manifests"
	"github.com/creydr/knative-emo-poc/pkg/manifests/transform"
	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/client/listers/messaging/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

type Reconciler struct {
	eventMeshLister       operatorv1alpha1listers.EventMeshLister
	manifest              mf.Manifest
	inMemoryChannelLister v1.InMemoryChannelLister
}

// Check that our Reconciler implements eventmeshreconciler.Interface
var _ eventmeshreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, em *v1alpha1.EventMesh) reconciler.Event {
	logger := logging.FromContext(ctx)
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

	return nil
}

func (r *Reconciler) applyScaling(manifests *knmf.Manifests) error {
	imcs, err := r.inMemoryChannelLister.List(labels.Everything())
	if err != nil {
		return fmt.Errorf("failed to list all InMemoryChannels: %w", err)
	}

	if len(imcs) == 0 {
		manifests.AddTransformers(
			transform.Scale(schema.FromAPIVersionAndKind("apps/v1", "Deployment"),
				"imc-dispatcher",
				"knative-eventing",
				0))
		manifests.AddTransformers(
			transform.Scale(schema.FromAPIVersionAndKind("apps/v1", "Deployment"),
				"imc-controller",
				"knative-eventing",
				0))
	} else {
		manifests.AddTransformers(
			transform.Scale(schema.FromAPIVersionAndKind("apps/v1", "Deployment"),
				"imc-dispatcher",
				"knative-eventing",
				1))
		manifests.AddTransformers(
			transform.Scale(schema.FromAPIVersionAndKind("apps/v1", "Deployment"),
				"imc-controller",
				"knative-eventing",
				1))
	}

	return nil
}
