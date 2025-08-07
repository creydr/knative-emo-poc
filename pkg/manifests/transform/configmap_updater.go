package transform

import (
	"fmt"

	mf "github.com/manifestival/manifestival"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
)

func ConfigMap(name, namespace, key, value string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConfigMap" {
			return nil
		}

		if u.GetNamespace() != namespace || u.GetName() != name {
			return nil
		}

		var cm = &corev1.ConfigMap{}
		if err := scheme.Scheme.Convert(u, cm, nil); err != nil {
			return fmt.Errorf("error converting unstructured to configmap: %w", err)
		}

		cm.Data[key] = value

		return scheme.Scheme.Convert(cm, u, nil)
	}
}

func ConfigMapMultipleValues(name, namespace string, keyValues map[string]string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConfigMap" {
			return nil
		}

		if u.GetNamespace() != namespace || u.GetName() != name {
			return nil
		}

		var cm = &corev1.ConfigMap{}
		if err := scheme.Scheme.Convert(u, cm, nil); err != nil {
			return fmt.Errorf("error converting unstructured to configmap: %w", err)
		}

		for k, v := range keyValues {
			cm.Data[k] = v
		}

		return scheme.Scheme.Convert(cm, u, nil)
	}
}
