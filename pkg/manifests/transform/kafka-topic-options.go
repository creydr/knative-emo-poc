package transform

import (
	"fmt"

	mf "github.com/manifestival/manifestival"
	"knative.dev/pkg/system"
)

func KafkaTopicOption(options map[string]string) mf.Transformer {
	const prefix = "default.topic.config."

	optionsWithPrefixedKeys := map[string]string{}
	for k, v := range options {
		optionsWithPrefixedKeys[fmt.Sprintf("%s%s", prefix, k)] = v
	}

	return ConfigMapMultipleValues(
		"kafka-broker-config",
		system.Namespace(),
		optionsWithPrefixedKeys,
		true)
}
