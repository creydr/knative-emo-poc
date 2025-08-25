package common

import (
	"context"

	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	"knative.dev/eventmesh-operator/pkg/manifests"
)

// Stage represents a step in the pipeline/stages list
type Stage func(context.Context, *manifests.Manifests, *v1alpha1.EventMesh) error

// Stages are a list of steps
type Stages []Stage

// Execute each stage in sequence until one returns an error
func (stages Stages) Execute(ctx context.Context, em *v1alpha1.EventMesh) error {
	m := &manifests.Manifests{}
	for _, stage := range stages {
		if err := stage(ctx, m, em); err != nil {
			if IsDeploymentsNotReadyError(err) {
				return nil
			}

			return err
		}
	}
	return nil
}
