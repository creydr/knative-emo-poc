package manifests

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	"knative.dev/pkg/logging"
)

func AppendFromParser(parser Parser) func(context.Context, *Manifests, *v1alpha1.EventMesh) error {
	return func(ctx context.Context, manifests *Manifests, em *v1alpha1.EventMesh) error {
		loadedManifests, err := parser.Parse(em)
		if err != nil {
			return fmt.Errorf("failed to get eventing manifests: %w", err)
		}
		manifests.Append(loadedManifests)

		return nil
	}
}

func Transform(ctx context.Context, manifests *Manifests, em *v1alpha1.EventMesh) error {
	logger := logging.FromContext(ctx)

	logger.Debug("Applying patches to manifests")
	if err := manifests.TransformToApply(); err != nil {
		return fmt.Errorf("failed to transform manifests to apply: %w", err)
	}

	// also run the transformers on the manifests which gets deleted, in case some metadata were patched before they were applied
	if err := manifests.TransformToDelete(); err != nil {
		return fmt.Errorf("failed to transform manifests to delete: %w", err)
	}

	return nil
}

func Install(baseManifest mf.Manifest) func(ctx context.Context, manifests *Manifests, em *v1alpha1.EventMesh) error {
	return func(ctx context.Context, manifests *Manifests, em *v1alpha1.EventMesh) error {
		logger := logging.FromContext(ctx)

		logger.Debug("Sort manifests for k8s order")
		manifests.Sort()

		// Delete old manifests
		logger.Debugf("Deleting unneeded manifests (%d)", len(manifests.ToDelete.Resources()))
		if err := baseManifest.Append(manifests.ToDelete).Delete(ctx, mf.IgnoreNotFound(true)); err != nil {
			return fmt.Errorf("failed to delete manifests: %w", err)
		}

		// Install manifests
		logger.Debugf("Applying manifests (%d)", len(manifests.ToApply.Resources()))
		if err := baseManifest.Append(manifests.ToApply).Apply(ctx); err != nil {
			return fmt.Errorf("failed to apply manifests: %w", err)
		}

		// TODO: apply post install

		return nil
	}
}
