package transform

import (
	mf "github.com/manifestival/manifestival"
	"knative.dev/pkg/system"
)

func FeatureFlags(features map[string]string) mf.Transformer {
	// TODO: we only set the eventing features, but EKB has features too (config-kafka-features)
	return ConfigMapMultipleValues(
		"config-features",
		system.Namespace(),
		features,
		false)
}
