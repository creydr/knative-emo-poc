package manifests

import (
	"fmt"
	"strings"

	mf "github.com/manifestival/manifestival"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	"knative.dev/eventmesh-operator/pkg/manifests/transform"
)

// ForEventing returns the configured manifests for eventing
func ForEventing(em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := &Manifests{}

	coreManifests, err := eventingCoreManifests(em)
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing core manifests: %w", err)
	}
	manifests.Append(coreManifests)

	// depending on EventMesh config, load additional manifests & Transformers (e.g. istio, TLS, ...)
	tlsManifests, err := eventingTLSManifests(em)
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing tls manifests: %w", err)
	}
	manifests.Append(tlsManifests)

	imcManifests, err := eventingIMCManifests(em)
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing IMC manifests: %w", err)
	}
	manifests.Append(imcManifests)

	mtBrokerManifests, err := eventingMTBrokerManifests(em)
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing MT channel Broker manifests: %w", err)
	}
	manifests.Append(mtBrokerManifests)

	// ...

	return manifests, nil
}

func eventingCoreManifests(em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	coreManifests, err := loadEventingCoreManifests()
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing core manifests: %w", err)
	}
	manifests.AddToApply(coreManifests)

	manifests.AddTransformers(transform.EventingCoreLogging(em.Spec.LogLevel))

	return &manifests, nil
}

func eventingTLSManifests(em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	tlsManifests, err := loadManifests("eventing-latest", "eventing-tls-networking.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing TLS manifests: %w", err)
	}

	if strings.EqualFold(em.Spec.TransportEncryption, string(feature.Strict)) ||
		strings.EqualFold(em.Spec.TransportEncryption, string(feature.Permissive)) {
		manifests.AddToApply(tlsManifests)

		// TODO: append feature configmap transformer
	} else {
		manifests.AddToDelete(tlsManifests)
	}

	return &manifests, nil
}

func eventingIMCManifests(em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	imcManifests, err := loadManifests("eventing-latest", "in-memory-channel.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing IMC manifests: %w", err)
	}

	// TODO: only add when enabled...
	manifests.AddToApply(imcManifests)

	return &manifests, nil
}

func eventingMTBrokerManifests(em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	mtBroker, err := loadManifests("eventing-latest", "mt-channel-broker.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load eventing MT channel broker manifests: %w", err)
	}

	// TODO: only add when enabled...
	manifests.AddToApply(mtBroker)

	return &manifests, nil
}

func loadEventingCoreManifests() (mf.Manifest, error) {
	eventingCoreFiles := []string{
		"eventing-crds.yaml",
		"eventing-core.yaml",
	}

	return loadManifests("eventing-latest", eventingCoreFiles...)
}
