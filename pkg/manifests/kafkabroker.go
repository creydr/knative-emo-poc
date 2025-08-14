package manifests

import (
	"fmt"
	"strings"

	mf "github.com/manifestival/manifestival"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	"knative.dev/eventmesh-operator/pkg/manifests/transform"
	"knative.dev/pkg/system"
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

	// we need a CM for the kafka-channel template
	manifests.AddToApply(eventingKafkaChannelTemplateConfigMap(em))

	manifests.AddTransformers(
		transform.KafkaLogging(em.Spec.LogLevel),
		transform.BootstrapServers(em.Spec.Kafka.BootstrapServers),
		transform.NumberOfPartitions(em.Spec.Kafka.NumPartitions),
		transform.ReplicationFactor(em.Spec.Kafka.ReplicationFactor),
		transform.KafkaTopicOption(em.Spec.Kafka.TopicConfigOptions),
	)

	return &manifests, nil
}

func eventingKafkaBrokerTLSManifests(em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	tlsManifests, err := loadManifests("eventing-kafka-broker-latest", "eventing-kafka-tls-networking.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load EKB TLS manifests: %w", err)
	}

	features, err := em.Spec.GetFeatureFlags()
	if err != nil {
		return nil, fmt.Errorf("failed to load feature flags: %w", err)
	}

	if !features.IsDisabledTransportEncryption() {
		manifests.AddToApply(tlsManifests)
	} else {
		manifests.AddToDelete(tlsManifests)
	}

	return &manifests, nil
}

func loadEventingKafkaBrokerCoreManifests() (mf.Manifest, error) {
	ekbCoreFiles := []string{
		"eventing-kafka-controller.yaml",
		"eventing-kafka-broker.yaml",
		"eventing-kafka-channel.yaml",
		"eventing-kafka-sink.yaml",
		"eventing-kafka-source.yaml",
	}

	return loadManifests("eventing-kafka-broker-latest", ekbCoreFiles...)
}

func eventingKafkaChannelTemplateConfigMap(em *v1alpha1.EventMesh) mf.Manifest {
	cm := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: kafka-channel
  namespace: %s
data:
  channel-template-spec: |
    apiVersion: messaging.knative.dev/v1
    kind: KafkaChannel
    spec:
      numPartitions: %d
      replicationFactor: %d`,
		system.Namespace(),
		em.Spec.Kafka.NumPartitions,
		em.Spec.Kafka.ReplicationFactor)

	reader := strings.NewReader(cm)

	m, err := mf.ManifestFrom(mf.Reader(reader))
	if err != nil {
		// this is an issue from the implementation side. OK to panic
		panic(fmt.Errorf("error parsing manifest for kafka channel template: %w", err))
	}
	return m
}
