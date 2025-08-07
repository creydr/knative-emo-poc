package transform

import (
	"fmt"

	mf "github.com/manifestival/manifestival"
	"knative.dev/pkg/system"
)

func NumberOfPartitions(num int32) mf.Transformer {
	return ConfigMap(
		"kafka-broker-config",
		system.Namespace(),
		"default.topic.partitions",
		fmt.Sprintf("%d", num))
}
