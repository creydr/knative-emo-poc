package manifests

import (
	"fmt"

	mf "github.com/manifestival/manifestival"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	"knative.dev/eventmesh-operator/pkg/manifests/transform"
)

func ForEventingKafkaBroker(em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := &Manifests{}

	coreManifests, err := eventingKafkaBrokerCoreManifests(em)
	if err != nil {
		return nil, fmt.Errorf("failed to load EKB core manifests: %w", err)
	}
	manifests.Append(coreManifests)

	// depending on EventMesh config, load additional manifests & Transformers
	// ...

	return manifests, nil
}

func eventingKafkaBrokerCoreManifests(em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	coreManifests, err := loadEventingKafkaBrokerCoreManifests()
	if err != nil {
		return nil, fmt.Errorf("failed to load EKB core manifests: %w", err)
	}
	manifests.AddToApply(coreManifests)

	manifests.AddTransformers(
		transform.KafkaLogging(em.Spec.LogLevel),
		transform.BootstrapServers(em.Spec.Kafka.BootstrapServers),
	)

	return &manifests, nil
}

func loadEventingKafkaBrokerCoreManifests() (mf.Manifest, error) {
	ekbCoreFiles := []string{
		"eventing-kafka-controller.yaml",
		"eventing-kafka-broker.yaml",
	}

	return loadManifests("eventing-kafka-broker-latest", ekbCoreFiles...)
}
