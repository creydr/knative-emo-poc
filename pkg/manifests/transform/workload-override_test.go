package transform

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
)

func TestWorkloadsOverride(t *testing.T) {
	tests := []struct {
		name      string
		overrides []v1alpha1.WorkloadOverride
		input     *unstructured.Unstructured
		expected  *unstructured.Unstructured
		wantErr   bool
	}{
		{
			name: "deployment with replicas override",
			overrides: []v1alpha1.WorkloadOverride{
				{
					Name:     "test-deployment",
					Replicas: ptr.To(int32(3)),
				},
			},
			input:    NewDeploymentBuilder("test-deployment").WithReplicas(1).Build(),
			expected: NewDeploymentBuilder("test-deployment").WithReplicas(3).Build(),
			wantErr:  false,
		},
		{
			name: "deployment with labels and annotations",
			overrides: []v1alpha1.WorkloadOverride{
				{
					Name: "test-deployment",
					Labels: map[string]string{
						"app": "test",
					},
					Annotations: map[string]string{
						"version": "v1.0.0",
					},
				},
			},
			input: NewDeploymentBuilder("test-deployment").WithReplicas(1).Build(),
			expected: NewDeploymentBuilder("test-deployment").
				WithReplicas(1).
				WithLabels(map[string]string{"app": "test"}).
				WithAnnotations(map[string]string{"version": "v1.0.0"}).
				WithPodLabels(map[string]string{"app": "test"}).
				WithPodAnnotations(map[string]string{"version": "v1.0.0"}).
				Build(),
			wantErr: false,
		},
		{
			name: "statefulset with replicas override",
			overrides: []v1alpha1.WorkloadOverride{
				{
					Name:     "test-statefulset",
					Replicas: ptr.To(int32(3)),
				},
			},
			input:    NewStatefulSetBuilder("test-statefulset").WithReplicas(1).Build(),
			expected: NewStatefulSetBuilder("test-statefulset").WithReplicas(3).Build(),
			wantErr:  false,
		},
		{
			name: "job with generate name match",
			overrides: []v1alpha1.WorkloadOverride{
				{
					Name: "test-job-",
					Labels: map[string]string{
						"batch": "true",
					},
				},
			},
			input: NewJobBuilder().WithGenerateName("test-job-").Build(),
			expected: NewJobBuilder().
				WithGenerateName("test-job-").
				WithLabels(map[string]string{"batch": "true"}).
				WithPodLabels(map[string]string{"batch": "true"}).
				Build(),
			wantErr: false,
		},
		{
			name: "deployment with HPA - replicas not overridden",
			overrides: []v1alpha1.WorkloadOverride{
				{
					Name:     "eventing-webhook",
					Replicas: ptr.To(int32(3)),
				},
			},
			input:    NewDeploymentBuilder("eventing-webhook").WithReplicas(1).Build(),
			expected: NewDeploymentBuilder("eventing-webhook").WithReplicas(1).Build(),
			wantErr:  false,
		},
		{
			name: "HPA transformation",
			overrides: []v1alpha1.WorkloadOverride{
				{
					Name:     "mt-broker-ingress",
					Replicas: ptr.To(int32(3)),
				},
			},
			input:    NewHPABuilder("broker-ingress-hpa").WithMinReplicas(1).WithMaxReplicas(5).Build(),
			expected: NewHPABuilder("broker-ingress-hpa").WithMinReplicas(3).WithMaxReplicas(7).Build(),
			wantErr:  false,
		},
		{
			name: "deployment with resource overrides",
			overrides: []v1alpha1.WorkloadOverride{
				{
					Name: "test-deployment",
					Resources: []v1alpha1.ResourceRequirementsOverride{
						{
							Container: "main",
							ResourceRequirements: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
						},
					},
				},
			},
			input: NewDeploymentBuilder("test-deployment").
				WithReplicas(1).
				WithContainerResources("main", "100m", "128Mi", "200m", "256Mi").
				Build(),
			expected: NewDeploymentBuilder("test-deployment").
				WithReplicas(1).
				WithContainerResources("main", "200m", "256Mi", "500m", "512Mi").
				Build(),
			wantErr: false,
		},
		{
			name: "deployment with env overrides",
			overrides: []v1alpha1.WorkloadOverride{
				{
					Name: "test-deployment",
					Env: []v1alpha1.EnvRequirementsOverride{
						{
							Container: "main",
							EnvVars: []corev1.EnvVar{
								{Name: "DEBUG", Value: "true"},
								{Name: "LOG_LEVEL", Value: "info"},
							},
						},
					},
				},
			},
			input: NewDeploymentBuilder("test-deployment").
				WithReplicas(1).
				WithContainerEnv("main", map[string]string{"EXISTING_VAR": "existing"}).
				Build(),
			expected: NewDeploymentBuilder("test-deployment").
				WithReplicas(1).
				WithContainerEnv("main", map[string]string{
					"EXISTING_VAR": "existing",
					"DEBUG":        "true",
					"LOG_LEVEL":    "info",
				}).
				Build(),
			wantErr: false,
		},
		{
			name: "deployment with node selector override",
			overrides: []v1alpha1.WorkloadOverride{
				{
					Name: "test-deployment",
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
						"node-type":        "compute",
					},
				},
			},
			input: NewDeploymentBuilder("test-deployment").WithReplicas(1).Build(),
			expected: NewDeploymentBuilder("test-deployment").
				WithReplicas(1).
				WithNodeSelector(map[string]string{
					"kubernetes.io/os": "linux",
					"node-type":        "compute",
				}).
				Build(),
			wantErr: false,
		},
		{
			name: "deployment with host network override",
			overrides: []v1alpha1.WorkloadOverride{
				{
					Name:        "test-deployment",
					HostNetwork: ptr.To(true),
				},
			},
			input: NewDeploymentBuilder("test-deployment").WithReplicas(1).Build(),
			expected: NewDeploymentBuilder("test-deployment").
				WithReplicas(1).
				WithHostNetwork(true).
				Build(),
			wantErr: false,
		},
		{
			name:      "no overrides returns nil transformer",
			overrides: nil,
			input:     NewDeploymentBuilder("test-deployment").WithReplicas(1).Build(),
			expected:  NewDeploymentBuilder("test-deployment").WithReplicas(1).Build(),
			wantErr:   false,
		},
		{
			name: "non-matching workload unchanged",
			overrides: []v1alpha1.WorkloadOverride{
				{
					Name:     "other-deployment",
					Replicas: ptr.To(int32(3)),
				},
			},
			input:    NewDeploymentBuilder("test-deployment").WithReplicas(1).Build(),
			expected: NewDeploymentBuilder("test-deployment").WithReplicas(1).Build(),
			wantErr:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transformer := WorkloadsOverride(test.overrides)
			if test.overrides == nil {
				if transformer != nil {
					t.Error("Expected nil transformer for nil overrides")
				}
				return
			}

			err := transformer(test.input)
			if (err != nil) != test.wantErr {
				t.Errorf("WorkloadsOverride() error = %v, wantErr %v", err, test.wantErr)
				return
			}

			if diff := cmp.Diff(test.expected.Object, test.input.Object); diff != "" {
				t.Errorf("WorkloadsOverride() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// DeploymentBuilder provides a fluent interface for building Deployment unstructured objects
type DeploymentBuilder struct {
	deployment *appsv1.Deployment
}

func NewDeploymentBuilder(name string) *DeploymentBuilder {
	return &DeploymentBuilder{
		deployment: &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Deployment",
				APIVersion: "apps/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: appsv1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "main",
								Image: "test:latest",
							},
						},
					},
				},
			},
		},
	}
}

func (b *DeploymentBuilder) WithReplicas(replicas int32) *DeploymentBuilder {
	b.deployment.Spec.Replicas = &replicas
	return b
}

func (b *DeploymentBuilder) WithLabels(labels map[string]string) *DeploymentBuilder {
	if b.deployment.Labels == nil {
		b.deployment.Labels = make(map[string]string)
	}
	for k, v := range labels {
		b.deployment.Labels[k] = v
	}
	return b
}

func (b *DeploymentBuilder) WithAnnotations(annotations map[string]string) *DeploymentBuilder {
	if b.deployment.Annotations == nil {
		b.deployment.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		b.deployment.Annotations[k] = v
	}
	return b
}

func (b *DeploymentBuilder) WithPodLabels(labels map[string]string) *DeploymentBuilder {
	if b.deployment.Spec.Template.Labels == nil {
		b.deployment.Spec.Template.Labels = make(map[string]string)
	}
	for k, v := range labels {
		b.deployment.Spec.Template.Labels[k] = v
	}
	return b
}

func (b *DeploymentBuilder) WithPodAnnotations(annotations map[string]string) *DeploymentBuilder {
	if b.deployment.Spec.Template.Annotations == nil {
		b.deployment.Spec.Template.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		b.deployment.Spec.Template.Annotations[k] = v
	}
	return b
}

func (b *DeploymentBuilder) WithNodeSelector(nodeSelector map[string]string) *DeploymentBuilder {
	b.deployment.Spec.Template.Spec.NodeSelector = nodeSelector
	return b
}

func (b *DeploymentBuilder) WithHostNetwork(hostNetwork bool) *DeploymentBuilder {
	b.deployment.Spec.Template.Spec.HostNetwork = hostNetwork
	if hostNetwork {
		b.deployment.Spec.Template.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
	}
	return b
}

func (b *DeploymentBuilder) WithContainerResources(containerName, requestCPU, requestMemory, limitCPU, limitMemory string) *DeploymentBuilder {
	for i := range b.deployment.Spec.Template.Spec.Containers {
		if b.deployment.Spec.Template.Spec.Containers[i].Name == containerName {
			b.deployment.Spec.Template.Spec.Containers[i].Resources = corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(requestCPU),
					corev1.ResourceMemory: resource.MustParse(requestMemory),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(limitCPU),
					corev1.ResourceMemory: resource.MustParse(limitMemory),
				},
			}
			break
		}
	}
	return b
}

func (b *DeploymentBuilder) WithContainerEnv(containerName string, envVars map[string]string) *DeploymentBuilder {
	for i := range b.deployment.Spec.Template.Spec.Containers {
		if b.deployment.Spec.Template.Spec.Containers[i].Name == containerName {
			var env []corev1.EnvVar
			for k, v := range envVars {
				env = append(env, corev1.EnvVar{Name: k, Value: v})
			}
			b.deployment.Spec.Template.Spec.Containers[i].Env = env
			break
		}
	}
	return b
}

func (b *DeploymentBuilder) Build() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	scheme.Scheme.Convert(b.deployment, u, nil)
	u.SetCreationTimestamp(metav1.Time{})
	return u
}

// StatefulSetBuilder provides a fluent interface for building StatefulSet unstructured objects
type StatefulSetBuilder struct {
	statefulSet *appsv1.StatefulSet
}

func NewStatefulSetBuilder(name string) *StatefulSetBuilder {
	return &StatefulSetBuilder{
		statefulSet: &appsv1.StatefulSet{
			TypeMeta: metav1.TypeMeta{
				Kind:       "StatefulSet",
				APIVersion: "apps/v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "main",
								Image: "test:latest",
							},
						},
					},
				},
			},
		},
	}
}

func (b *StatefulSetBuilder) WithReplicas(replicas int32) *StatefulSetBuilder {
	b.statefulSet.Spec.Replicas = &replicas
	return b
}

func (b *StatefulSetBuilder) Build() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	scheme.Scheme.Convert(b.statefulSet, u, nil)
	u.SetCreationTimestamp(metav1.Time{})
	return u
}

// JobBuilder provides a fluent interface for building Job unstructured objects
type JobBuilder struct {
	job *batchv1.Job
}

func NewJobBuilder() *JobBuilder {
	return &JobBuilder{
		job: &batchv1.Job{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Job",
				APIVersion: "batch/v1",
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "main",
								Image: "test:latest",
							},
						},
					},
				},
			},
		},
	}
}

func (b *JobBuilder) WithGenerateName(generateName string) *JobBuilder {
	b.job.GenerateName = generateName
	return b
}

func (b *JobBuilder) WithLabels(labels map[string]string) *JobBuilder {
	if b.job.Labels == nil {
		b.job.Labels = make(map[string]string)
	}
	for k, v := range labels {
		b.job.Labels[k] = v
	}
	return b
}

func (b *JobBuilder) WithPodLabels(labels map[string]string) *JobBuilder {
	if b.job.Spec.Template.Labels == nil {
		b.job.Spec.Template.Labels = make(map[string]string)
	}
	for k, v := range labels {
		b.job.Spec.Template.Labels[k] = v
	}
	return b
}

func (b *JobBuilder) Build() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	scheme.Scheme.Convert(b.job, u, nil)
	u.SetCreationTimestamp(metav1.Time{})
	return u
}

// HPABuilder provides a fluent interface for building HPA unstructured objects
type HPABuilder struct {
	hpa *autoscalingv2.HorizontalPodAutoscaler
}

func NewHPABuilder(name string) *HPABuilder {
	return &HPABuilder{
		hpa: &autoscalingv2.HorizontalPodAutoscaler{
			TypeMeta: metav1.TypeMeta{
				Kind:       "HorizontalPodAutoscaler",
				APIVersion: "autoscaling/v2",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		},
	}
}

func (b *HPABuilder) WithMinReplicas(minReplicas int32) *HPABuilder {
	b.hpa.Spec.MinReplicas = &minReplicas
	return b
}

func (b *HPABuilder) WithMaxReplicas(maxReplicas int32) *HPABuilder {
	b.hpa.Spec.MaxReplicas = maxReplicas
	return b
}

func (b *HPABuilder) Build() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	scheme.Scheme.Convert(b.hpa, u, nil)
	u.SetCreationTimestamp(metav1.Time{})
	return u
}
