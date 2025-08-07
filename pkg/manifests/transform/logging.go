package transform

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	mf "github.com/manifestival/manifestival"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
)

func EventingCoreLogging(logLevel string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ConfigMap" {
			return nil
		}

		if u.GetNamespace() != system.Namespace() || u.GetName() != logging.ConfigMapName() {
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
		zapConfig, err := zapConfigFromJSON(cm.Data["zap-logger-config"])
		if err != nil {
			return fmt.Errorf("error parsing zap logging config: %w", err)
		}

		zapConfig.Level = zap.NewAtomicLevelAt(newLoggingLevel)
		zapConfigJSON, err := zapConfigToJSON(zapConfig)
		if err != nil {
			return fmt.Errorf("error converting zap logging config to JSON: %w", err)
		}

		cm.Data["zap-logger-config"] = zapConfigJSON

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

		// Get the XML configuration
		configXML, exists := cm.Data["config.xml"]
		if !exists {
			return fmt.Errorf("config.xml not found in kafka-config-logging configmap")
		}

		// Convert EventMesh log level to logback format
		logbackLevel, err := convertToLogbackLogLevel(logLevel)
		if err != nil {
			return fmt.Errorf("error converting log level: %w", err)
		}

		// Update the root logger level using regex
		// This matches: <root level="CURRENT_LEVEL"> and replaces the level value
		// Regex explanation:
		// - (<root\s+level=") captures the opening part with optional whitespace
		// - [^"]+ matches the current level value (any non-quote characters)
		// - (") captures the closing quote
		rootLevelRegex := regexp.MustCompile(`(<root\s+level=")[^"]+(")`)
		if !rootLevelRegex.MatchString(configXML) {
			return fmt.Errorf("could not find <root level=\"...\"> pattern in logging configuration")
		}

		updatedXML := rootLevelRegex.ReplaceAllString(configXML, "${1}"+logbackLevel+"${2}")

		// Verify the replacement worked
		if updatedXML == configXML {
			return fmt.Errorf("log level replacement failed - XML unchanged")
		}

		// Update the configmap
		cm.Data["config.xml"] = updatedXML

		return scheme.Scheme.Convert(cm, u, nil)
	}
}

func zapConfigFromJSON(configJSON string) (*zap.Config, error) {
	// Start with a base production config that has all function encoders properly set
	loggingCfg := zap.NewProductionConfig()

	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &loggingCfg); err != nil {
			return nil, err
		}
	}

	return &loggingCfg, nil
}

func zapConfigToJSON(config *zap.Config) (string, error) {
	// Create a custom encoder config struct with only serializable fields
	type SerializableEncoderConfig struct {
		MessageKey       string `json:"messageKey,omitempty"`
		LevelKey         string `json:"levelKey,omitempty"`
		TimeKey          string `json:"timeKey,omitempty"`
		NameKey          string `json:"nameKey,omitempty"`
		CallerKey        string `json:"callerKey,omitempty"`
		FunctionKey      string `json:"functionKey,omitempty"`
		StacktraceKey    string `json:"stacktraceKey,omitempty"`
		SkipLineEnding   bool   `json:"skipLineEnding,omitempty"`
		LineEnding       string `json:"lineEnding,omitempty"`
		ConsoleSeparator string `json:"consoleSeparator,omitempty"`
	}

	// Create a simplified config struct that contains only JSON-serializable fields
	simplifiedConfig := struct {
		Level             string                    `json:"level"`
		Development       bool                      `json:"development"`
		DisableCaller     bool                      `json:"disableCaller"`
		DisableStacktrace bool                      `json:"disableStacktrace"`
		Encoding          string                    `json:"encoding"`
		OutputPaths       []string                  `json:"outputPaths"`
		ErrorOutputPaths  []string                  `json:"errorOutputPaths"`
		InitialFields     map[string]interface{}    `json:"initialFields,omitempty"`
		EncoderConfig     SerializableEncoderConfig `json:"encoderConfig"`
	}{
		Level:             config.Level.String(),
		Development:       config.Development,
		DisableCaller:     config.DisableCaller,
		DisableStacktrace: config.DisableStacktrace,
		Encoding:          config.Encoding,
		OutputPaths:       config.OutputPaths,
		ErrorOutputPaths:  config.ErrorOutputPaths,
		InitialFields:     config.InitialFields,
		EncoderConfig: SerializableEncoderConfig{
			MessageKey:       config.EncoderConfig.MessageKey,
			LevelKey:         config.EncoderConfig.LevelKey,
			TimeKey:          config.EncoderConfig.TimeKey,
			NameKey:          config.EncoderConfig.NameKey,
			CallerKey:        config.EncoderConfig.CallerKey,
			FunctionKey:      config.EncoderConfig.FunctionKey,
			StacktraceKey:    config.EncoderConfig.StacktraceKey,
			SkipLineEnding:   config.EncoderConfig.SkipLineEnding,
			LineEnding:       config.EncoderConfig.LineEnding,
			ConsoleSeparator: config.EncoderConfig.ConsoleSeparator,
		},
	}

	b, err := json.MarshalIndent(simplifiedConfig, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error converting config to JSON: %w", err)
	}

	return string(b), nil
}

// sanitizeEncoderConfig removes function types that cannot be marshaled to JSON
func sanitizeEncoderConfig(cfg zapcore.EncoderConfig) zapcore.EncoderConfig {
	// Create a copy and set only the serializable fields
	sanitized := zapcore.EncoderConfig{
		MessageKey:       cfg.MessageKey,
		LevelKey:         cfg.LevelKey,
		TimeKey:          cfg.TimeKey,
		NameKey:          cfg.NameKey,
		CallerKey:        cfg.CallerKey,
		FunctionKey:      cfg.FunctionKey,
		StacktraceKey:    cfg.StacktraceKey,
		SkipLineEnding:   cfg.SkipLineEnding,
		LineEnding:       cfg.LineEnding,
		ConsoleSeparator: cfg.ConsoleSeparator,
		// Note: We deliberately omit the function encoder fields:
		// EncodeLevel, EncodeTime, EncodeDuration, EncodeCaller, EncodeName
		// These will be restored from the base production config when loading
	}
	return sanitized
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

func convertToLogbackLogLevel(logLevel string) (string, error) {
	if slices.Contains(v1alpha1.LogLevels, strings.ToLower(logLevel)) {
		return strings.ToUpper(logLevel), nil
	}

	return "", fmt.Errorf("unknown log level %s", logLevel)
}
