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
	"os"

	"github.com/creydr/knative-emo-poc/pkg/apis/operator/v1alpha1"
	eventmeshreconciler "github.com/creydr/knative-emo-poc/pkg/client/injection/reconciler/operator/v1alpha1/eventmesh"
	operatorv1alpha1listers "github.com/creydr/knative-emo-poc/pkg/client/listers/operator/v1alpha1"
	"github.com/creydr/knative-emo-poc/pkg/reconciler/eventmesh/transform"
	mf "github.com/manifestival/manifestival"
	"knative.dev/pkg/reconciler"
)

type Reconciler struct {
	eventMeshLister operatorv1alpha1listers.EventMeshLister
	manifest        mf.Manifest
}

// Check that our Reconciler implements eventmeshreconciler.Interface
var _ eventmeshreconciler.Interface = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, em *v1alpha1.EventMesh) reconciler.Event {
	// Get eventing manifests
	eventingManifests, err := r.eventingManifests(em)
	if err != nil {
		return fmt.Errorf("failed to get eventing manifests: %w", err)
	}

	// Get EKB manifests
	ekbManifests, err := r.eventingKafkaBrokerManifests(em)
	if err != nil {
		return fmt.Errorf("failed to get EKB manifests: %w", err)
	}

	// Apply manifests
	if err := r.manifest.Append(eventingManifests, ekbManifests).Apply(); err != nil {
		return fmt.Errorf("failed to apply manifests: %w", err)
	}

	return nil
}

//TODO: move manifests funcs into separate package

func (r *Reconciler) eventingManifests(em *v1alpha1.EventMesh) (mf.Manifest, error) {
	manifests := mf.Manifest{}
	var transformers []mf.Transformer

	// core manifests
	coreManifests, err := r.loadEventingCoreManifests()
	if err != nil {
		return mf.Manifest{}, fmt.Errorf("failed to load eventing core manifests: %w", err)
	}
	manifests = manifests.Append(coreManifests)
	transformers = append(transformers, transform.EventingCoreLogging(em.Spec.LogLevel))

	// depending on EventMesh config, load additional manifests & Transformers (e.g. istio, TLS, ...)

	// Patch manifests
	patchedManifests, err := manifests.Transform(transformers...)
	if err != nil {
		return mf.Manifest{}, fmt.Errorf("failed to transform eventing manifests: %w", err)
	}

	return patchedManifests, nil
}

func (r *Reconciler) loadEventingCoreManifests() (mf.Manifest, error) {
	eventingCoreFiles := []string{
		"eventing-crds.yaml",
		"eventing-core.yaml",
	}

	return r.loadManifests("eventing-latest", eventingCoreFiles...)
}

func (r *Reconciler) eventingKafkaBrokerManifests(em *v1alpha1.EventMesh) (mf.Manifest, error) {
	manifests := mf.Manifest{}
	var transformers []mf.Transformer

	coreManifests, err := r.loadEventingKafkaBrokerCoreManifests()
	if err != nil {
		return mf.Manifest{}, fmt.Errorf("failed to load EKB core manifests: %w", err)
	}
	manifests = manifests.Append(coreManifests)
	transformers = append(transformers, transform.KafkaLogging(em.Spec.LogLevel))

	// depending on EventMesh config, load additional manifests & Transformers (e.g. TLS, bootstrap, ...)
	// ...

	// Patch manifests
	patchedManifests, err := manifests.Transform(transformers...)
	if err != nil {
		return mf.Manifest{}, fmt.Errorf("failed to transform EKB manifests: %w", err)
	}

	return patchedManifests, nil
}

func (r *Reconciler) loadEventingKafkaBrokerCoreManifests() (mf.Manifest, error) {
	ekbCoreFiles := []string{
		"eventing-kafka-controller.yaml",
		"eventing-kafka-broker.yaml",
	}

	return r.loadManifests("eventing-kafka-broker-latest", ekbCoreFiles...)
}

func (r *Reconciler) loadManifests(dirname string, filenames ...string) (mf.Manifest, error) {
	manifests := mf.Manifest{}
	for _, file := range filenames {
		manifest, err := mf.NewManifest(fmt.Sprintf("%s/%s/%s", os.Getenv("KO_DATA_PATH"), dirname, file))
		if err != nil {
			return mf.Manifest{}, fmt.Errorf("failed to parse EKB manifest %s: %w", file, err)
		}

		manifests = manifests.Append(manifest)
	}

	return manifests, nil
}
