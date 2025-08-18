package transform

import (
	"fmt"

	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Scale(gvk schema.GroupVersionKind, name string, namespace string, replicas int64) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		scalableKinds := map[schema.GroupVersionKind]bool{
			schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}:        true,
			schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}:       true,
			schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "ReplicaSet"}:        true,
			schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ReplicationController"}: true,
		}

		if !scalableKinds[u.GroupVersionKind()] {
			return nil
		}

		if !scalableKinds[gvk] {
			return fmt.Errorf("%s is not a scalable kind", gvk.String())
		}

		if u.GroupVersionKind().Group != gvk.Group ||
			u.GroupVersionKind().Version != gvk.Version ||
			u.GroupVersionKind().Kind != gvk.Kind {
			return nil
		}

		if u.GetNamespace() != namespace || u.GetName() != name {
			return nil
		}

		// Set .spec.replicas field
		return unstructured.SetNestedField(u.Object, replicas, "spec", "replicas")
	}
}
