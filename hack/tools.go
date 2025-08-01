//go:build tools
// +build tools

package tools

import (
	_ "k8s.io/code-generator"
	_ "knative.dev/hack"
	_ "knative.dev/pkg/hack"

	_ "k8s.io/code-generator/cmd/validation-gen"
)
