package transform

import (
	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	"knative.dev/pkg/system"
)

func EventingFeatureFlags(features *v1alpha1.EventMeshSpecFeatures) mf.Transformer {
	if features != nil && features.Eventing != nil {
		return ConfigMapMultipleValues(
			"config-features",
			system.Namespace(),
			features.Eventing,
			false)
	}

	return NoOp()
}

func EventingKafkaBrokerFeatureFlags(features *v1alpha1.EventMeshSpecFeatures) mf.Transformer {
	if features != nil && features.EventingKafkaBroker != nil {
		return ConfigMapMultipleValues(
			"config-kafka-features",
			system.Namespace(),
			features.EventingKafkaBroker,
			false)
	}

	return NoOp()
}

func NoOp() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		return nil
	}
}
