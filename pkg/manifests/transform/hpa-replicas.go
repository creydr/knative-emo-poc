package transform

import (
	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// HPAReplicas sets the minReplicas and maxReplicas of an HPA based on a replica override value.
// If minReplica needs to be increased, the maxReplica is increased by the same value.
func HPAReplicas(name, namespace string, minReplicas int64) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "HorizontalPodAutoscaler" || u.GetNamespace() != namespace || u.GetName() != name {
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
}
