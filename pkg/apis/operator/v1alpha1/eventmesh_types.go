package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

const (
	LogLevelTrace = "trace"
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
	LogLevelFatal = "fatal"
)

var (
	LogLevels = []string{
		LogLevelTrace,
		LogLevelDebug,
		LogLevelInfo,
		LogLevelWarn,
		LogLevelError,
		LogLevelFatal,
	}
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventMesh represents the EventMesh
type EventMesh struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec EventMeshSpec `json:"spec,omitempty"`

	// +optional
	Status EventMeshStatus `json:"status,omitempty"`
}

type EventMeshSpec struct {
	Kafka               EventMeshSpecKafka `json:"kafka,omitempty"`
	LogLevel            string             `json:"logLevel,omitempty"`
	TransportEncryption string             `json:"transportEncryption,omitempty"`
}

type EventMeshSpecKafka struct {
	BootstrapServers []string `json:"bootstrapServers,omitempty"`

	// +optional
	AuthSecretRef     *corev1.SecretReference `json:"authSecretRef,omitempty"`
	NumPartitions     int32                   `json:"numPartitions,omitempty"`
	ReplicationFactor int32                   `json:"replicationFactor,omitempty"`
}

type EventMeshStatus struct {
	// inherits duck/v1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EventMeshList is a collection of EventMesh.
type EventMeshList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EventMesh `json:"items"`
}

var (
	// Check that EventMesh can be validated, can be defaulted, and has immutable fields.
	_ apis.Validatable = (*EventMesh)(nil)
	_ apis.Defaultable = (*EventMesh)(nil)

	// Check that EventMesh can return its spec untyped.
	_ apis.HasSpec = (*EventMesh)(nil)

	_ runtime.Object = (*EventMesh)(nil)

	// Check that we can create OwnerReferences to an EventMesh.
	_ kmeta.OwnerRefable = (*EventMesh)(nil)

	// Check that the type conforms to the duck Knative Resource shape.
	_ duckv1.KRShaped = (*EventMesh)(nil)
)

// GetGroupVersionKind returns GroupVersionKind for EventMesh
func (em *EventMesh) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("EventMesh")
}

// GetUntypedSpec returns the spec of the EventMesh.
func (em *EventMesh) GetUntypedSpec() interface{} {
	return em.Spec
}

// GetStatus retrieves the status of the EventMesh. Implements the KRShaped interface.
func (em *EventMesh) GetStatus() *duckv1.Status {
	return &em.Status.Status
}
