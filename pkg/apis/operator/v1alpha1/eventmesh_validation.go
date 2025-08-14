package v1alpha1

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"knative.dev/pkg/apis"
)

func (em *EventMesh) Validate(ctx context.Context) *apis.FieldError {
	return em.Spec.Validate(ctx).ViaField("spec")
}

func (spec *EventMeshSpec) Validate(ctx context.Context) *apis.FieldError {
	var err *apis.FieldError

	if spec.LogLevel != "" && !slices.Contains(LogLevels, spec.LogLevel) {
		err = err.Also(apis.ErrInvalidValue(spec.LogLevel, "logLevel", fmt.Sprintf("must be one of %q", strings.Join(LogLevels, ", "))))
	}

	if spec.DefaultBroker != "" && !slices.Contains(BrokerClasses, spec.DefaultBroker) {
		err = err.Also(apis.ErrInvalidValue(spec.DefaultBroker, "defaultBroker", fmt.Sprintf("must be one of %q", strings.Join(BrokerClasses, ", "))))
	}

	if spec.DefaultChannel != "" && !slices.Contains(ChannelImplementations, spec.DefaultChannel) {
		err = err.Also(apis.ErrInvalidValue(spec.DefaultChannel, "defaultChannel", fmt.Sprintf("must be one of %q", strings.Join(ChannelImplementations, ", "))))
	}

	err = err.Also(spec.Kafka.Validate(ctx).ViaField("kafka"))

	return err
}

func (kafka *EventMeshSpecKafka) Validate(ctx context.Context) *apis.FieldError {
	var err *apis.FieldError

	if len(kafka.BootstrapServers) == 0 {
		err = err.Also(apis.ErrMissingField("bootstrapServers"))
	}

	if kafka.NumPartitions < 0 {
		err = err.Also(apis.ErrInvalidValue(kafka.NumPartitions, "numPartitions"))
	}

	if kafka.ReplicationFactor < 0 {
		err = err.Also(apis.ErrInvalidValue(kafka.ReplicationFactor, "replicationFactor"))
	}

	return err
}
