package manifests

import (
	context "context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	"knative.dev/eventmesh-operator/pkg/manifests/transform"
	"knative.dev/operator/pkg/reconciler/common"
	"knative.dev/pkg/logging"
)

// ForEventing returns the configured manifests for eventing
func ForEventing(ctx context.Context, em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := &Manifests{}

	coreManifests, err := eventingCoreManifests(ctx, em)
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing core manifests: %w", err)
	}
	manifests.Append(coreManifests)

	imcManifests, err := eventingIMCManifests(ctx, em)
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing IMC manifests: %w", err)
	}
	manifests.Append(imcManifests)

	mtBrokerManifests, err := eventingMTBrokerManifests(ctx, em)
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing MT channel Broker manifests: %w", err)
	}
	manifests.Append(mtBrokerManifests)

	tlsManifests, err := eventingTLSManifests(ctx, em)
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing tls manifests: %w", err)
	}
	manifests.Append(tlsManifests)

	// ...

	return manifests, nil
}

func eventingCoreManifests(ctx context.Context, em *v1alpha1.EventMesh) (*Manifests, error) {
	logger := logging.FromContext(ctx)
	manifests := Manifests{}

	coreManifests, err := loadEventingCoreManifests()
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing core manifests: %w", err)
	}
	manifests.AddToApply(coreManifests)

	manifests.AddTransformers(
		transform.EventingCoreLogging(em.Spec.LogLevel),
		transform.DefaultChannelImplementation(em.Spec.DefaultChannel),
		transform.DefaultBrokerClass(em.Spec.DefaultBroker, em.Spec.DefaultChannel),
		transform.FeatureFlags(em.Spec.Features),
		common.OverridesTransform(em.Spec.Overrides.Workloads, logger),
		common.ConfigMapTransform(em.Spec.Overrides.Config, logger))

	return &manifests, nil
}

func eventingIMCManifests(ctx context.Context, em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	imcManifests, err := loadManifests("eventing-latest", "in-memory-channel.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing IMC manifests: %w", err)
	}

	// we install the manifests by default, but scale the deployments down when no IMC exists
	manifests.AddToApply(imcManifests)

	return &manifests, nil
}

func eventingMTBrokerManifests(ctx context.Context, em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	mtBroker, err := loadManifests("eventing-latest", "mt-channel-broker.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing MT channel broker manifests: %w", err)
	}

	// we install the manifests by default, but scale the deployments down when no MTCB broker exists
	manifests.AddToApply(mtBroker)

	return &manifests, nil
}

func eventingTLSManifests(ctx context.Context, em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	tlsManifests, err := loadManifests("eventing-latest", "eventing-tls-networking.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing TLS manifests: %w", err)
	}

	features, err := em.Spec.GetFeatureFlags()
	if err != nil {
		return nil, fmt.Errorf("failed to load feature flags: %w", err)
	}

	if !features.IsDisabledTransportEncryption() {
		// here we need certmanager too
		manifests.AddToApply(tlsManifests)
	} else {
		manifests.AddToDelete(tlsManifests)
	}

	return &manifests, nil
}

func loadEventingCoreManifests() (mf.Manifest, error) {
	eventingCoreFiles := []string{
		"eventing-crds.yaml",
		"eventing-core.yaml",
	}

	return loadManifests("eventing-latest", eventingCoreFiles...)
}
