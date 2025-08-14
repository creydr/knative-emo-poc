package transform

import (
	"fmt"

	mf "github.com/manifestival/manifestival"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/eventing/pkg/apis/config"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	"knative.dev/pkg/system"
)

func DefaultBrokerClass(brokerClass, channel string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConfigMap" {
			return nil
		}

		if u.GetNamespace() != system.Namespace() || u.GetName() != config.DefaultsConfigName {
			return nil
		}

		var cm = &corev1.ConfigMap{}
		if err := scheme.Scheme.Convert(u, cm, nil); err != nil {
			return fmt.Errorf("error converting unstructured to configmap: %w", err)
		}

		if brokerClass == "" {
			brokerClass = v1alpha1.BrokerClassKafka
		}

		var alternativeBrokerClass string
		switch brokerClass {
		case v1alpha1.BrokerClassKafka:
			alternativeBrokerClass = v1alpha1.BrokerClassMTChannelBased
		case v1alpha1.BrokerClassMTChannelBased:
			alternativeBrokerClass = v1alpha1.BrokerClassKafka
		}

		defaults := fmt.Sprintf(`clusterDefault:
  brokerClass: %s
  apiVersion: v1
  kind: ConfigMap
  name: %s
  namespace: %s
  brokerClasses:
    %s:
      apiVersion: v1
      kind: ConfigMap
      name: %s
      namespace: %s`,
			brokerClass,
			configMapForBrokerClass(brokerClass, channel),
			system.Namespace(),
			alternativeBrokerClass,
			configMapForBrokerClass(alternativeBrokerClass, channel),
			system.Namespace())

		cm.Data[config.BrokerDefaultsKey] = defaults

		return scheme.Scheme.Convert(cm, u, nil)
	}
}

func configMapForBrokerClass(brokerClass, channel string) string {
	if brokerClass == v1alpha1.BrokerClassMTChannelBased {
		if channel == v1alpha1.ChannelImplementationIMC {
			return "config-br-default-channel"
		}
		return "kafka-channel"
	}

	return "kafka-broker-config"
}
