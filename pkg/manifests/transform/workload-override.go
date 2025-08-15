package transform

import (
	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
)

// Source copied from https://github.com/knative/operator/blob/650497b2493703ac73d5b50ef2fbf70e5dc57bba/pkg/reconciler/common/workload_override.go#L33
// due to dependency issues

// WorkloadsOverride transforms deployments based on the configuration in `spec.overrides`.
func WorkloadsOverride(overrides []v1alpha1.WorkloadOverride) mf.Transformer {
	if overrides == nil {
		return nil
	}
	return func(u *unstructured.Unstructured) error {
		for _, override := range overrides {
			var obj metav1.Object
			var ps *corev1.PodTemplateSpec

			if u.GetKind() == "Deployment" && u.GetName() == override.Name {
				deployment := &appsv1.Deployment{}
				if err := scheme.Scheme.Convert(u, deployment, nil); err != nil {
					return err
				}
				obj = deployment
				ps = &deployment.Spec.Template

				// Do not set replicas, if this resource is controlled by a HPA
				if override.Replicas != nil && !hasHorizontalPodOrCustomAutoscaler(override.Name) {
					deployment.Spec.Replicas = override.Replicas
				}
			}
			if u.GetKind() == "StatefulSet" && u.GetName() == override.Name {
				ss := &appsv1.StatefulSet{}
				if err := scheme.Scheme.Convert(u, ss, nil); err != nil {
					return err
				}
				obj = ss
				ps = &ss.Spec.Template

				// Do not set replicas, if this resource is controlled by a HPA
				if override.Replicas != nil && !hasHorizontalPodOrCustomAutoscaler(override.Name) {
					ss.Spec.Replicas = override.Replicas
				}
			}
			if u.GetKind() == "Job" && u.GetGenerateName() == override.Name {
				job := &batchv1.Job{}
				if err := scheme.Scheme.Convert(u, job, nil); err != nil {
					return err
				}
				obj = job
				ps = &job.Spec.Template
			}

			if u.GetKind() == "HorizontalPodAutoscaler" && override.Replicas != nil && u.GetName() == getHPAName(override.Name) {
				overrideReplicas := int64(*override.Replicas)
				if err := hpaTransform(u, overrideReplicas); err != nil {
					return err
				}
			}

			if obj == nil {
				continue
			}

			replaceLabels(&override, obj, ps)
			replaceAnnotations(&override, obj, ps)
			replaceNodeSelector(&override, ps)
			replaceTopologySpreadConstraints(&override, ps)
			replaceTolerations(&override, ps)
			replaceAffinities(&override, ps)
			replaceResources(&override, ps)
			replaceEnv(&override, ps)
			replaceProbes(&override, ps)
			replaceHostNetwork(&override, ps)

			if err := scheme.Scheme.Convert(obj, u, nil); err != nil {
				return err
			}

			// Avoid superfluous updates from converted zero defaults
			u.SetCreationTimestamp(metav1.Time{})
		}
		return nil
	}
}

// When a Podspecable has HPA or a custom autoscaling, the replicas should be controlled by it instead of operator.
// Hence, skip changing the spec.replicas for these Podspecables.
func hasHorizontalPodOrCustomAutoscaler(name string) bool {
	return sets.NewString(
		"eventing-webhook",
		"mt-broker-ingress",
		"mt-broker-filter",
		"kafka-broker-dispatcher",
		"kafka-source-dispatcher",
		"kafka-channel-dispatcher",
	).Has(name)
}

// Maps a Podspecables name to the HPAs name.
// Add overrides here, if your HPA is named differently to the workloads name,
// if no override is defined, the name of the podspecable is used as HPA name.
func getHPAName(podspecableName string) string {
	overrides := map[string]string{
		"mt-broker-ingress": "broker-ingress-hpa",
		"mt-broker-filter":  "broker-filter-hpa",
	}
	if v, ok := overrides[podspecableName]; ok {
		return v
	} else {
		return podspecableName
	}
}

// hpaTransform sets the minReplicas and maxReplicas of an HPA based on a replica override value.
// If minReplica needs to be increased, the maxReplica is increased by the same value.
func hpaTransform(u *unstructured.Unstructured, minReplicas int64) error {
	if u.GetKind() != "HorizontalPodAutoscaler" {
		return nil
	}

	min, _, err := unstructured.NestedInt64(u.Object, "spec", "minReplicas")
	if err != nil {
		return err
	}

	if err := unstructured.SetNestedField(u.Object, minReplicas, "spec", "minReplicas"); err != nil {
		return err
	}

	max, found, err := unstructured.NestedInt64(u.Object, "spec", "maxReplicas")
	if err != nil {
		return err
	}

	// Do nothing if maxReplicas is not defined.
	if !found {
		return nil
	}

	// Increase maxReplicas to the amount that we increased,
	// because we need to avoid minReplicas > maxReplicas happening.
	if err := unstructured.SetNestedField(u.Object, max+(minReplicas-min), "spec", "maxReplicas"); err != nil {
		return err
	}
	return nil
}

func replaceAnnotations(override *v1alpha1.WorkloadOverride, obj metav1.Object, ps *corev1.PodTemplateSpec) {
	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(map[string]string{})
	}
	if ps.GetAnnotations() == nil {
		ps.SetAnnotations(map[string]string{})
	}
	for key, val := range override.Annotations {
		obj.GetAnnotations()[key] = val
		ps.Annotations[key] = val
	}
}

func replaceLabels(override *v1alpha1.WorkloadOverride, obj metav1.Object, ps *corev1.PodTemplateSpec) {
	if obj.GetLabels() == nil {
		obj.SetLabels(map[string]string{})
	}
	if ps.GetLabels() == nil {
		ps.Labels = map[string]string{}
	}
	for key, val := range override.Labels {
		obj.GetLabels()[key] = val
		ps.Labels[key] = val
	}
}

func replaceNodeSelector(override *v1alpha1.WorkloadOverride, ps *corev1.PodTemplateSpec) {
	if len(override.NodeSelector) > 0 {
		ps.Spec.NodeSelector = override.NodeSelector
	}
}

func replaceTopologySpreadConstraints(override *v1alpha1.WorkloadOverride, ps *corev1.PodTemplateSpec) {
	if len(override.TopologySpreadConstraints) > 0 {
		ps.Spec.TopologySpreadConstraints = override.TopologySpreadConstraints
	}
}

func replaceTolerations(override *v1alpha1.WorkloadOverride, ps *corev1.PodTemplateSpec) {
	if len(override.Tolerations) > 0 {
		ps.Spec.Tolerations = override.Tolerations
	}
}

func replaceAffinities(override *v1alpha1.WorkloadOverride, ps *corev1.PodTemplateSpec) {
	if override.Affinity != nil {
		ps.Spec.Affinity = override.Affinity
	}
}

func replaceResources(override *v1alpha1.WorkloadOverride, ps *corev1.PodTemplateSpec) {
	if len(override.Resources) > 0 {
		containers := ps.Spec.Containers
		for i := range containers {
			if override := find(override.Resources, containers[i].Name); override != nil {
				merge(&override.Limits, &containers[i].Resources.Limits)
				merge(&override.Requests, &containers[i].Resources.Requests)
			}
		}
	}
}

func replaceEnv(override *v1alpha1.WorkloadOverride, ps *corev1.PodTemplateSpec) {
	if len(override.Env) > 0 {
		containers := ps.Spec.Containers
		for i := range containers {
			if override := findEnvOverride(override.Env, containers[i].Name); override != nil {
				mergeEnv(&override.EnvVars, &containers[i].Env)
			}
		}
	}
}

func replaceProbes(override *v1alpha1.WorkloadOverride, ps *corev1.PodTemplateSpec) {
	if len(override.ReadinessProbes) > 0 {
		containers := ps.Spec.Containers
		for i := range containers {
			override := findProbeOverride(override.ReadinessProbes, containers[i].Name)
			if override != nil {
				overrideProbe := &corev1.Probe{
					InitialDelaySeconds:           override.InitialDelaySeconds,
					TimeoutSeconds:                override.TimeoutSeconds,
					PeriodSeconds:                 override.PeriodSeconds,
					SuccessThreshold:              override.SuccessThreshold,
					FailureThreshold:              override.FailureThreshold,
					TerminationGracePeriodSeconds: override.TerminationGracePeriodSeconds,
				}
				if *overrideProbe == (corev1.Probe{}) {
					//  Disable probe when users explicitly set the empty overrideProbe.
					containers[i].ReadinessProbe = nil
					continue
				}
				if containers[i].ReadinessProbe == nil {
					containers[i].ReadinessProbe = overrideProbe
					continue
				}
				mergeProbe(overrideProbe, containers[i].ReadinessProbe)
			}
		}
	}

	if len(override.LivenessProbes) > 0 {
		containers := ps.Spec.Containers
		for i := range containers {
			if override := findProbeOverride(override.LivenessProbes, containers[i].Name); override != nil {
				overrideProbe := &corev1.Probe{
					InitialDelaySeconds:           override.InitialDelaySeconds,
					TimeoutSeconds:                override.TimeoutSeconds,
					PeriodSeconds:                 override.PeriodSeconds,
					SuccessThreshold:              override.SuccessThreshold,
					FailureThreshold:              override.FailureThreshold,
					TerminationGracePeriodSeconds: override.TerminationGracePeriodSeconds,
				}
				if *overrideProbe == (corev1.Probe{}) {
					//  Disable probe when users explicitly set the empty overrideProbe.
					containers[i].LivenessProbe = nil
					continue
				}
				if containers[i].LivenessProbe == nil {
					containers[i].LivenessProbe = overrideProbe
					continue
				}
				mergeProbe(overrideProbe, containers[i].LivenessProbe)
			}
		}
	}
}

func replaceHostNetwork(override *v1alpha1.WorkloadOverride, ps *corev1.PodTemplateSpec) {
	if override.HostNetwork != nil {
		ps.Spec.HostNetwork = *override.HostNetwork

		if *override.HostNetwork {
			ps.Spec.DNSPolicy = corev1.DNSClusterFirstWithHostNet
		}
	}
}

func merge(src, tgt *corev1.ResourceList) {
	if len(*tgt) > 0 {
		for k, v := range *src {
			(*tgt)[k] = v
		}
	} else {
		*tgt = *src
	}
}

func find(resources []v1alpha1.ResourceRequirementsOverride, name string) *v1alpha1.ResourceRequirementsOverride {
	for _, override := range resources {
		if override.Container == name {
			return &override
		}
	}
	return nil
}

func mergeEnv(src, tgt *[]corev1.EnvVar) {
	if len(*tgt) > 0 {
		for _, srcV := range *src {
			exists := false
			for i, tgtV := range *tgt {
				if srcV.Name == tgtV.Name {
					(*tgt)[i] = srcV
					exists = true
				}
			}
			if !exists {
				*tgt = append(*tgt, srcV)
			}
		}
	} else {
		*tgt = *src
	}
}

func findEnvOverride(resources []v1alpha1.EnvRequirementsOverride, name string) *v1alpha1.EnvRequirementsOverride {
	for _, override := range resources {
		if override.Container == name {
			return &override
		}
	}
	return nil
}

func mergeProbe(override, tgt *corev1.Probe) {
	if override == nil {
		return
	}
	var merged corev1.Probe
	jtgt, _ := json.Marshal(*tgt)
	_ = json.Unmarshal(jtgt, &merged)
	jsrc, _ := json.Marshal(*override)
	_ = json.Unmarshal(jsrc, &merged)
	jmerged, _ := json.Marshal(merged)
	_ = json.Unmarshal(jmerged, tgt)
}

func findProbeOverride(probes []v1alpha1.ProbesRequirementsOverride, name string) *v1alpha1.ProbesRequirementsOverride {
	for _, override := range probes {
		if override.Container == name {
			return &override
		}
	}
	return nil
}
