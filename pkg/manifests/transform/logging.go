package transform

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	mf "github.com/manifestival/manifestival"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	"knative.dev/pkg/system"
)

func EventingCoreLogging(logLevel string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConfigMap" {
			return nil
		}

		if u.GetNamespace() != system.Namespace() || u.GetName() != "config-logging" {
			return nil
		}

		var cm = &corev1.ConfigMap{}
		if err := scheme.Scheme.Convert(u, cm, nil); err != nil {
			return fmt.Errorf("error converting unstructured to configmap: %w", err)
		}

		newLoggingLevel, err := convertToZapLogLevel(logLevel)
		if err != nil {
			return fmt.Errorf("error parsing target log level: %w", err)
		}

		// set component wise logging level
		for key := range cm.Data {
			if strings.HasPrefix(key, "loglevel.") {
				cm.Data[key] = newLoggingLevel.String()
			}
		}

		// update logging level in zap config
		if err := updateZapConfigLevel(cm.Data, newLoggingLevel.String()); err != nil {
			return fmt.Errorf("error updating zap logging config: %w", err)
		}

		return scheme.Scheme.Convert(cm, u, nil)
	}
}

func KafkaLogging(logLevel string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConfigMap" {
			return nil
		}

		if u.GetNamespace() != system.Namespace() || u.GetName() != "kafka-config-logging" {
			return nil
		}

		var cm = &corev1.ConfigMap{}
		if err := scheme.Scheme.Convert(u, cm, nil); err != nil {
			return fmt.Errorf("error converting unstructured to configmap: %w", err)
		}

		if err := updateKafkaLoggingConfig(cm.Data, logLevel); err != nil {
			return fmt.Errorf("error updating kafka logging config: %w", err)
		}

		return scheme.Scheme.Convert(cm, u, nil)
	}
}

func updateKafkaLoggingConfig(data map[string]string, logLevel string) error {
	configXML, exists := data["config.xml"]
	if !exists {
		return fmt.Errorf("config.xml not found in kafka-config-logging configmap")
	}

	logbackLevel := strings.ToUpper(logLevel)
	updatedXML := configXML

	rootLevelPattern := regexp.MustCompile(`(<root\s+[^>]*level=")[^"]+("(?:[^>]*>))`)
	updatedXML = rootLevelPattern.ReplaceAllString(updatedXML, "${1}"+logbackLevel+"${2}")

	loggerLevelPattern := regexp.MustCompile(`(<logger\s+[^>]*level=")[^"]+("(?:[^>]*>))`)
	updatedXML = loggerLevelPattern.ReplaceAllString(updatedXML, "${1}"+logbackLevel+"${2}")

	if updatedXML == configXML {
		return fmt.Errorf("no logger or root level elements found in XML configuration")
	}

	data["config.xml"] = updatedXML
	return nil
}

func updateZapConfigLevel(data map[string]string, level string) error {
	zapConfigJSON, exists := data["zap-logger-config"]
	if !exists {
		// Create a default zap config if it doesn't exist
		defaultConfig := map[string]interface{}{
			"level":            level,
			"development":      false,
			"outputPaths":      []string{"stdout"},
			"errorOutputPaths": []string{"stderr"},
			"encoding":         "json",
		}
		configBytes, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return err
		}
		data["zap-logger-config"] = string(configBytes)
		return nil
	}

	// Parse existing config and update the level
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(zapConfigJSON), &config); err != nil {
		return err
	}

	config["level"] = level

	// Marshal back to JSON
	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	data["zap-logger-config"] = string(configBytes)
	return nil
}

func convertToZapLogLevel(logLevel string) (zapcore.Level, error) {
	switch strings.ToLower(logLevel) {
	case v1alpha1.LogLevelTrace:
		fallthrough
	case v1alpha1.LogLevelDebug:
		return zapcore.DebugLevel, nil
	case v1alpha1.LogLevelInfo:
		return zapcore.InfoLevel, nil
	case v1alpha1.LogLevelWarn:
		return zapcore.WarnLevel, nil
	case v1alpha1.LogLevelError:
		return zapcore.ErrorLevel, nil
	case v1alpha1.LogLevelFatal:
		return zapcore.FatalLevel, nil
	}
	return zapcore.InvalidLevel, fmt.Errorf("unknown log level %s", logLevel)
}
