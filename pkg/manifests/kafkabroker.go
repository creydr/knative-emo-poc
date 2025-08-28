package manifests

import (
	"context"
	"fmt"
	"strings"

	mf "github.com/manifestival/manifestival"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	"knative.dev/eventmesh-operator/pkg/manifests/transform"
	"knative.dev/eventmesh-operator/pkg/utils"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
)

// kafkaBrokerParser parses Kafka broker manifests
type kafkaBrokerParser struct {
	crdLister        apiextensionsv1.CustomResourceDefinitionLister
	deploymentLister appsv1listers.DeploymentLister
}

// NewKafkaBrokerParser creates a new Kafka broker manifest parser
func NewKafkaBrokerParser(crdLister apiextensionsv1.CustomResourceDefinitionLister, deploymentLister appsv1listers.DeploymentLister) Parser {
	return &kafkaBrokerParser{
		crdLister:        crdLister,
		deploymentLister: deploymentLister,
	}
}

// Parse returns the configured manifests for Kafka broker
func (p *kafkaBrokerParser) Parse(ctx context.Context, em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := &Manifests{}

	coreManifests, err := p.eventingKafkaBrokerCoreManifests(em)
	if err != nil {
		return nil, fmt.Errorf("failed to load EKB core manifests: %w", err)
	}
	manifests.Append(coreManifests)

	tlsManifests, err := p.eventingKafkaBrokerTLSManifests(em)
	if err != nil {
		return nil, fmt.Errorf("failed to load EKB tls manifests: %w", err)
	}
	manifests.Append(tlsManifests)

	postInstallManifests, err := p.eventingKafkaBrokerPostInstallManifests(ctx, em, coreManifests)
	if err != nil {
		return nil, fmt.Errorf("failed to load EKB post-install manifests: %w", err)
	}
	manifests.Append(postInstallManifests)

	return manifests, nil
}

func (p *kafkaBrokerParser) eventingKafkaBrokerCoreManifests(em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	coreManifests, err := p.loadEventingKafkaBrokerCoreManifests()
	if err != nil {
		return nil, fmt.Errorf("failed to load EKB core manifests: %w", err)
	}
	manifests.AddToApply(coreManifests)

	// we need a CM for the kafka-channel template
	manifests.AddToApply(p.eventingKafkaChannelTemplateConfigMap(em))

	manifests.AddTransformers(
		transform.KafkaLogging(em.Spec.LogLevel),
		transform.BootstrapServers(em.Spec.Kafka.BootstrapServers),
		transform.NumberOfPartitions(em.Spec.Kafka.NumPartitions),
		transform.ReplicationFactor(em.Spec.Kafka.ReplicationFactor),
		transform.KafkaTopicOption(em.Spec.Kafka.TopicConfigOptions),
	)

	return &manifests, nil
}

func (p *kafkaBrokerParser) eventingKafkaBrokerTLSManifests(em *v1alpha1.EventMesh) (*Manifests, error) {
	manifests := Manifests{}

	tlsManifests, err := loadManifests("eventing-kafka-broker-latest", "eventing-kafka-tls-networking.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load EKB TLS manifests: %w", err)
	}

	certManagerIsInstalled, err := utils.IsCertmanagerInstalled(p.crdLister)
	if err != nil {
		return nil, fmt.Errorf("failed to check if cert-manager is installed: %w", err)
	}

	features, err := em.Spec.GetFeatureFlags()
	if err != nil {
		return nil, fmt.Errorf("failed to load feature flags: %w", err)
	}

	if !features.IsDisabledTransportEncryption() {
		if certManagerIsInstalled {
			manifests.AddToApply(tlsManifests)
		} else {
			return nil, fmt.Errorf("%s is set to %s, but cert-manager is not installed", feature.TransportEncryption, features[feature.TransportEncryption])
		}
	} else {
		if certManagerIsInstalled {
			manifests.AddToDelete(tlsManifests)
		}
	}

	return &manifests, nil
}

func (p *kafkaBrokerParser) eventingKafkaBrokerPostInstallManifests(ctx context.Context, em *v1alpha1.EventMesh, manifests *Manifests) (*Manifests, error) {
	logger := logging.FromContext(ctx)

	upgrade, err := isUpgrade(manifests.ToApply, p.deploymentLister)
	if err != nil {
		return nil, fmt.Errorf("failed to check if this is an upgrade: %w", err)
	}

	if upgrade {
		logger.Debug("Adding post-install manifests because at least minor version of manifests are upgraded")
		// we only install post-install manifests on updates

		postInstallManifests, err := loadManifests("eventing-kafka-broker-latest", "eventing-kafka-post-install.yaml")
		if err != nil {
			return nil, fmt.Errorf("failed to load EKB post-install manifests: %w", err)
		}

		result := Manifests{}
		result.AddToPostInstall(postInstallManifests)

		return &result, nil
	}
	logger.Debug("Skipping to add post-install manifests, as no (at least) minor version upgrade is ongoing")

	return nil, nil
}

func (p *kafkaBrokerParser) loadEventingKafkaBrokerCoreManifests() (mf.Manifest, error) {
	ekbCoreFiles := []string{
		"eventing-kafka-controller.yaml",
		"eventing-kafka-broker.yaml",
		"eventing-kafka-channel.yaml",
		"eventing-kafka-sink.yaml",
		"eventing-kafka-source.yaml",
	}

	return loadManifests("eventing-kafka-broker-latest", ekbCoreFiles...)
}

func (p *kafkaBrokerParser) eventingKafkaChannelTemplateConfigMap(em *v1alpha1.EventMesh) mf.Manifest {
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
