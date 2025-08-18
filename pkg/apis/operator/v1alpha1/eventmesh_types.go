package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/eventing/pkg/apis/feature"
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

	BrokerClassKafka          = "Kafka"
	BrokerClassMTChannelBased = "MTChannelBasedBroker"

	ChannelImplementationKafka = "KafkaChannel"
	ChannelImplementationIMC   = "InMemoryChannel"
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

	BrokerClasses = []string{
		BrokerClassKafka,
		BrokerClassMTChannelBased,
	}

	ChannelImplementations = []string{
		ChannelImplementationKafka,
		ChannelImplementationIMC,
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
	Kafka EventMeshSpecKafka `json:"kafka,omitempty"`

	// +optional
	LogLevel string `json:"logLevel,omitempty"`

	// +optional
	DefaultBroker string `json:"defaultBroker,omitempty"`

	// +optional
	DefaultChannel string `json:"defaultChannel,omitempty"`

	// +optional
	Features map[string]string `json:"features,omitempty"`

	// +optional
	Overrides *EventMeshSpecOverrides `json:"overrides,omitempty"`
}

type EventMeshSpecKafka struct {
	BootstrapServers []string `json:"bootstrapServers,omitempty"`

	// +optional
	AuthSecretRef *corev1.LocalObjectReference `json:"authSecretRef,omitempty"`

	// +optional
	NumPartitions int32 `json:"numPartitions,omitempty"`

	// +optional
	ReplicationFactor int32 `json:"replicationFactor,omitempty"`

	// +optional
	TopicConfigOptions map[string]string `json:"topicConfigOptions,omitempty"`
}

type EventMeshSpecOverrides struct {
	// +optional
	Config map[string]map[string]string `json:"config,omitempty"`

	// +optional
	Workloads []WorkloadOverride `json:"workloads,omitempty"`
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

func (ems *EventMeshSpec) GetFeatureFlags() (feature.Flags, error) {
	return feature.NewFlagsConfigFromMap(ems.Features)
}

// WorkloadOverride copied from https://github.com/knative/operator/blob/650497b2493703ac73d5b50ef2fbf70e5dc57bba/pkg/apis/operator/base/common.go#L274
// due to dependency issues

// WorkloadOverride defines the configurations of deployments to override.
type WorkloadOverride struct {
	// Name is the name of the deployment to override.
	Name string `json:"name"`

	// Labels overrides labels for the deployment and its template.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations overrides labels for the deployment and its template.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Replicas is the number of replicas that HA parts of the control plane
	// will be scaled to.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// NodeSelector overrides nodeSelector for the deployment.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// TopologySpreadConstraints overrides topologySpreadConstraints for the deployment.
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// Tolerations overrides tolerations for the deployment.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinities overrides affinity for the deployment.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// Resources overrides resources for the containers.
	// +optional
	Resources []ResourceRequirementsOverride `json:"resources,omitempty"`

	// Env overrides env vars for the containers.
	// +optional
	Env []EnvRequirementsOverride `json:"env,omitempty"`

	// ReadinessProbes overrides readiness probes for the containers.
	// +optional
	ReadinessProbes []ProbesRequirementsOverride `json:"readinessProbes,omitempty"`

	// LivenessProbes overrides liveness probes for the containers.
	// +optional
	LivenessProbes []ProbesRequirementsOverride `json:"livenessProbes,omitempty"`

	// HostNetwork overrides hostNetwork for the containers.
	// When hostNetwork is enabled, this will set dnsPolicy to ClusterFirstWithHostNet automatically for the containers.
	// +optional
	HostNetwork *bool `json:"hostNetwork,omitempty"`
}

// ResourceRequirementsOverride enables the user to override any container's
// resource requests/limits specified in the embedded manifest
type ResourceRequirementsOverride struct {
	// The container name
	Container string `json:"container"`
	// The desired ResourceRequirements
	corev1.ResourceRequirements
}

// EnvRequirementsOverride enables the user to override any container's env vars.
type EnvRequirementsOverride struct {
	// The container name
	Container string `json:"container"`
	// The desired EnvVarRequirements
	EnvVars []corev1.EnvVar `json:"envVars,omitempty"`
}

// ProbesRequirementsOverride enables the user to override any container's env vars.
type ProbesRequirementsOverride struct {
	// The container name
	Container string `json:"container"`
	// Number of seconds after the container has started before liveness probes are initiated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	InitialDelaySeconds int32 `json:"initialDelaySeconds,omitempty" protobuf:"varint,2,opt,name=initialDelaySeconds"`
	// Number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	// +optional
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty" protobuf:"varint,3,opt,name=timeoutSeconds"`
	// How often (in seconds) to perform the probe.
	// Default to 10 seconds. Minimum value is 1.
	// +optional
	PeriodSeconds int32 `json:"periodSeconds,omitempty" protobuf:"varint,4,opt,name=periodSeconds"`
	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// Defaults to 1. Must be 1 for liveness and startup. Minimum value is 1.
	// +optional
	SuccessThreshold int32 `json:"successThreshold,omitempty" protobuf:"varint,5,opt,name=successThreshold"`
	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	// +optional
	FailureThreshold int32 `json:"failureThreshold,omitempty" protobuf:"varint,6,opt,name=failureThreshold"`
	// Optional duration in seconds the pod needs to terminate gracefully upon probe failure.
	// The grace period is the duration in seconds after the processes running in the pod are sent
	// a termination signal and the time when the processes are forcibly halted with a kill signal.
	// Set this value longer than the expected cleanup time for your process.
	// If this value is nil, the pod's terminationGracePeriodSeconds will be used. Otherwise, this
	// value overrides the value provided by the pod spec.
	// Value must be non-negative integer. The value zero indicates stop immediately via
	// the kill signal (no opportunity to shut down).
	// This is a beta field and requires enabling ProbeTerminationGracePeriod feature gate.
	// Minimum value is 1. spec.terminationGracePeriodSeconds is used if unset.
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty" protobuf:"varint,7,opt,name=terminationGracePeriodSeconds"`
}
