package v1alpha1

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"knative.dev/pkg/apis"
)

func TestEventMeshSpecValidationLogLevel(t *testing.T) {
	tests := []struct {
		name string
		em   *EventMesh
		want *apis.FieldError
	}{
		{
			name: "valid log level",
			em: &EventMesh{
				Spec: EventMeshSpec{
					Kafka: EventMeshSpecKafka{
						BootstrapServers: []string{
							"server-1",
						},
					},
					LogLevel: LogLevelDebug,
				},
			},
			want: func() *apis.FieldError {
				return nil
			}(),
		},
		{
			name: "invalid, unknown log level",
			em: &EventMesh{
				Spec: EventMeshSpec{
					Kafka: EventMeshSpecKafka{
						BootstrapServers: []string{
							"server-1",
						},
					},
					LogLevel: "ultra",
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrInvalidValue("ultra", "spec.logLevel", fmt.Sprintf("must be one of %q", strings.Join(LogLevels, ", ")))
			}(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := apis.WithinCreate(context.TODO())
			got := test.em.Validate(ctx)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate EventMeshSpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestEventMeshSpecValidationBrokerClass(t *testing.T) {
	tests := []struct {
		name string
		em   *EventMesh
		want *apis.FieldError
	}{
		{
			name: "valid broker class",
			em: &EventMesh{
				Spec: EventMeshSpec{
					Kafka: EventMeshSpecKafka{
						BootstrapServers: []string{
							"server-1",
						},
					},
					DefaultBroker: BrokerClassKafka,
				},
			},
			want: func() *apis.FieldError {
				return nil
			}(),
		},
		{
			name: "invalid, unknown broker class",
			em: &EventMesh{
				Spec: EventMeshSpec{
					Kafka: EventMeshSpecKafka{
						BootstrapServers: []string{
							"server-1",
						},
					},
					DefaultBroker: "foobar",
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrInvalidValue("foobar", "spec.defaultBroker", fmt.Sprintf("must be one of %q", strings.Join(BrokerClasses, ", ")))
			}(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := apis.WithinCreate(context.TODO())
			got := test.em.Validate(ctx)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate EventMeshSpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestEventMeshSpecValidationChannelImplementation(t *testing.T) {
	tests := []struct {
		name string
		em   *EventMesh
		want *apis.FieldError
	}{
		{
			name: "valid channel implementation",
			em: &EventMesh{
				Spec: EventMeshSpec{
					Kafka: EventMeshSpecKafka{
						BootstrapServers: []string{
							"server-1",
						},
					},
					DefaultChannel: ChannelImplementationKafka,
				},
			},
			want: func() *apis.FieldError {
				return nil
			}(),
		},
		{
			name: "invalid, unknown channel implementation",
			em: &EventMesh{
				Spec: EventMeshSpec{
					Kafka: EventMeshSpecKafka{
						BootstrapServers: []string{
							"server-1",
						},
					},
					DefaultChannel: "foobar",
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrInvalidValue("foobar", "spec.defaultChannel", fmt.Sprintf("must be one of %q", strings.Join(ChannelImplementations, ", ")))
			}(),
		},
		{
			name: "invalid, checks on case",
			em: &EventMesh{
				Spec: EventMeshSpec{
					Kafka: EventMeshSpecKafka{
						BootstrapServers: []string{
							"server-1",
						},
					},
					DefaultChannel: "kafka",
				},
			},
			want: func() *apis.FieldError {
				return apis.ErrInvalidValue("kafka", "spec.defaultChannel", fmt.Sprintf("must be one of %q", strings.Join(ChannelImplementations, ", ")))
			}(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := apis.WithinCreate(context.TODO())
			got := test.em.Validate(ctx)
			if diff := cmp.Diff(test.want.Error(), got.Error()); diff != "" {
				t.Errorf("%s: Validate EventMeshSpec (-want, +got) = %v", test.name, diff)
			}
		})
	}
}
