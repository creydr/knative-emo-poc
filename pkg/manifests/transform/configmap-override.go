package transform

import (
	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Source code copied from https://github.com/knative/operator/blob/650497b2493703ac73d5b50ef2fbf70e5dc57bba/pkg/reconciler/common/config_maps.go
// due to dependency issues

// ConfigMapOverride updates the ConfigMaps
func ConfigMapOverride(config map[string]map[string]string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// Let any config in instance override everything else
		if u.GetKind() == "ConfigMap" {
			if data, ok := config[u.GetName()]; ok {
				return updateConfigMap(u, data)
			}
			// The "config-" prefix is optional
			if data, ok := config[u.GetName()[len(`config-`):]]; ok {
				return updateConfigMap(u, data)
			}
		}
		return nil
	}
}

// updateConfigMap set some data in a configmap, only overwriting common keys if they differ
func updateConfigMap(cm *unstructured.Unstructured, data map[string]string) error {
	for k, v := range data {
		x, found, err := unstructured.NestedFieldNoCopy(cm.Object, "data", k)
		if err != nil {
			return err
		}
		if found {
			if v == x {
				continue
			}
		}
		if err := unstructured.SetNestedField(cm.Object, v, "data", k); err != nil {
			return err
		}
	}
	return nil
}
