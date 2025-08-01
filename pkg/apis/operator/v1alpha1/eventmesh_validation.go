package v1alpha1

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"knative.dev/eventing/pkg/apis/feature"
	"knative.dev/pkg/apis"
)

func (em *EventMesh) Validate(ctx context.Context) *apis.FieldError {
	return em.Spec.Validate(ctx).ViaField("spec")
}

func (spec *EventMeshSpec) Validate(ctx context.Context) *apis.FieldError {
	var err *apis.FieldError

	if !slices.Contains(LogLevels, spec.LogLevel) {
		err = err.Also(apis.ErrInvalidValue(spec.LogLevel, "logLevel", fmt.Sprintf("must be one of %q", strings.Join(LogLevels, ", "))))
	}

	if spec.TransportEncryption != "" &&
		!strings.EqualFold(spec.TransportEncryption, string(feature.Disabled)) &&
		!strings.EqualFold(spec.TransportEncryption, string(feature.Permissive)) &&
		!strings.EqualFold(spec.TransportEncryption, string(feature.Strict)) {
		err = err.Also(apis.ErrInvalidValue(spec.TransportEncryption, "transportEncryption"))
	}

	err = err.Also(spec.Kafka.Validate(ctx).ViaField("kafka"))

	return err
}

func (kafka *EventMeshSpecKafka) Validate(ctx context.Context) *apis.FieldError {
	var err *apis.FieldError

	if len(kafka.BootstrapServers) == 0 {
		err = err.Also(apis.ErrMissingField("bootstrapServers"))
	}

	return err
}
