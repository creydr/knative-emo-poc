package transform

import (
	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// InjectOwner injects the owner to resources except into namespaces
func InjectOwner(owner mf.Owner) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Namespace" {
			return nil
		}

		return mf.InjectOwner(owner)(u)
	}
}
