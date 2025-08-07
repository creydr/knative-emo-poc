package manifestival

import (
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// LessFunc is a comparison function that returns true if resource a should be ordered before resource b.
// It follows the same convention as Go's sort package comparison functions.
type LessFunc func(a, b unstructured.Unstructured) bool

func (m Manifest) Sort(lessFunc LessFunc) Manifest {
	result := m
	// Make a copy of the resources slice to avoid modifying the original
	result.resources = make([]unstructured.Unstructured, len(m.resources))
	copy(result.resources, m.resources)

	sort.Slice(result.resources, func(i, j int) bool {
		return lessFunc(result.resources[i], result.resources[j])
	})

	return result
}

func ByKindPriority() LessFunc {
	return func(a, b unstructured.Unstructured) bool {
		kindA := a.GetKind()
		kindB := b.GetKind()

		posA := getKindPosition(kindA)
		posB := getKindPosition(kindB)

		// If positions are different, sort by position
		if posA != posB {
			return posA < posB
		}

		// If positions are the same (both unknown), sort alphabetically by kind
		return kindA < kindB
	}
}

// kindPriorityOrder defines the order in which Kubernetes resources should be deployed.
// Resources earlier in the list should be deployed before resources later in the list.
// Based on: https://github.com/helm/helm/blob/3c5d68d62e00e11a3efdf6c4b9d2d07272ae0c75/pkg/release/util/kind_sorter.go#L31
var kindPriorityOrder = []string{
	"PriorityClass",
	"Namespace",
	"NetworkPolicy",
	"ResourceQuota",
	"LimitRange",
	"PodSecurityPolicy",
	"PodDisruptionBudget",
	"ServiceAccount",
	"Secret",
	"SecretList",
	"ConfigMap",
	"StorageClass",
	"PersistentVolume",
	"PersistentVolumeClaim",
	"CustomResourceDefinition",
	"ClusterRole",
	"ClusterRoleList",
	"ClusterRoleBinding",
	"ClusterRoleBindingList",
	"Role",
	"RoleList",
	"RoleBinding",
	"RoleBindingList",
	"Service",
	"DaemonSet",
	"Pod",
	"ReplicationController",
	"ReplicaSet",
	"Deployment",
	"HorizontalPodAutoscaler",
	"StatefulSet",
	"Job",
	"CronJob",
	"IngressClass",
	"Ingress",
	"APIService",
	"MutatingWebhookConfiguration",
	"ValidatingWebhookConfiguration",
}

func getKindPosition(kind string) int {
	for i, k := range kindPriorityOrder {
		if k == kind {
			return i
		}
	}
	return len(kindPriorityOrder) // Unknown kinds go last
}
