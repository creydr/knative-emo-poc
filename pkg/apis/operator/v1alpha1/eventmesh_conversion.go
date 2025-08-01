package v1alpha1

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible
func (em *EventMesh) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	return fmt.Errorf("v1alpha1 is the highest known version, got: %T", obj)
}

// ConvertFrom implements apis.Convertible
func (em *EventMesh) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	return fmt.Errorf("v1alpha1 is the highest known version, got: %T", obj)
}
