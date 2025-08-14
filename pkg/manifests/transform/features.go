package transform

import (
	mf "github.com/manifestival/manifestival"
	"knative.dev/pkg/system"
)

func FeatureFlags(features map[string]string) mf.Transformer {
	return ConfigMapMultipleValues(
		"config-features",
		system.Namespace(),
		features,
		false)
}
