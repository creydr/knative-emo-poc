package transform

import (
	"fmt"

	mf "github.com/manifestival/manifestival"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/eventing/pkg/apis/messaging/config"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	"knative.dev/pkg/system"
)

func DefaultChannelImplementation(channel string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConfigMap" {
			return nil
		}

		if u.GetNamespace() != system.Namespace() || u.GetName() != config.ChannelDefaultsConfigName {
			return nil
		}

		var cm = &corev1.ConfigMap{}
		if err := scheme.Scheme.Convert(u, cm, nil); err != nil {
			return fmt.Errorf("error converting unstructured to configmap: %w", err)
		}

		if channel == "" {
			channel = v1alpha1.ChannelImplementationKafka
		}

		defaults := fmt.Sprintf(`clusterDefault:
  apiVersion: messaging.knative.dev/v1
  kind: %s`,
			channel)

		cm.Data[config.ChannelDefaulterKey] = defaults

		return scheme.Scheme.Convert(cm, u, nil)
	}
}
