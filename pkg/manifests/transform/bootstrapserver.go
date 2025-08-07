package transform

import (
	"fmt"
	"strings"

	mf "github.com/manifestival/manifestival"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/pkg/system"
)

func BootstrapServers(servers []string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConfigMap" {
			return nil
		}

		if u.GetNamespace() != system.Namespace() || (u.GetName() != "kafka-broker-config" && u.GetName() != "kafka-channel-config") {
			return nil
		}

		var cm = &corev1.ConfigMap{}
		if err := scheme.Scheme.Convert(u, cm, nil); err != nil {
			return fmt.Errorf("error converting unstructured to configmap: %w", err)
		}

		cm.Data["bootstrap.servers"] = strings.Join(servers, ",")

		return scheme.Scheme.Convert(cm, u, nil)
	}
}
