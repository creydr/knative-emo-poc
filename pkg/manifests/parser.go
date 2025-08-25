package manifests

import (
	"context"

	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
)

// Parser parses manifests for different components
type Parser interface {
	Parse(ctx context.Context, em *v1alpha1.EventMesh) (*Manifests, error)
}
