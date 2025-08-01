package v1alpha1

import (
	"context"
	"strings"

	"knative.dev/pkg/apis"
)

func (em *EventMesh) SetDefaults(ctx context.Context) {
	ctx = apis.WithinParent(ctx, em.ObjectMeta)
	em.Spec.SetDefaults(ctx)
}

func (spec *EventMeshSpec) SetDefaults(ctx context.Context) {
	spec.Kafka.SetDefaults(ctx)

	spec.LogLevel = strings.ToLower(spec.LogLevel)
	if spec.LogLevel == "" {
		spec.LogLevel = "info"
	}
}

func (kafka *EventMeshSpecKafka) SetDefaults(ctx context.Context) {
	if kafka.NumPartitions <= 0 {
		kafka.NumPartitions = 3
	}

	if kafka.ReplicationFactor <= 0 {
		kafka.ReplicationFactor = 1
	}
}
