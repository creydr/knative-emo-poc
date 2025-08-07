package transform

import (
	"fmt"

	mf "github.com/manifestival/manifestival"
	"knative.dev/pkg/system"
)

func ReplicationFactor(num int32) mf.Transformer {
	return ConfigMap(
		"kafka-broker-config",
		system.Namespace(),
		"default.topic.replication.factor",
		fmt.Sprintf("%d", num))
}
