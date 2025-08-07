package main

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/pkg/webhook"
	"knative.dev/pkg/webhook/certificates"
	"knative.dev/pkg/webhook/resourcesemantics"
	"knative.dev/pkg/webhook/resourcesemantics/defaulting"
	"knative.dev/pkg/webhook/resourcesemantics/validation"

	eventmeshv1alpha1 "knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
	versionedscheme "knative.dev/eventmesh-operator/pkg/client/clientset/versioned/scheme"
)

func init() {
	versionedscheme.AddToScheme(scheme.Scheme)
}

var ourTypes = map[schema.GroupVersionKind]resourcesemantics.GenericCRD{
	eventmeshv1alpha1.SchemeGroupVersion.WithKind("EventMesh"): &eventmeshv1alpha1.EventMesh{},
}

var callbacks = map[schema.GroupVersionKind]validation.Callback{}

func NewDefaultingAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	ctxFunc := func(ctx context.Context) context.Context {
		return ctx
	}

	return defaulting.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"webhook.eventmesh.knative.dev",

		// The path on which to serve the webhook.
		"/defaulting",

		// The resources to default.
		ourTypes,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		ctxFunc,

		// Whether to disallow unknown fields.
		true,
	)
}

func NewValidationAdmissionController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	ctxFunc := func(ctx context.Context) context.Context {
		return ctx
	}

	return validation.NewAdmissionController(ctx,

		// Name of the resource webhook.
		"validation.webhook.eventmesh.knative.dev",

		// The path on which to serve the webhook.
		"/resource-validation",

		// The resources to validate.
		ourTypes,

		// A function that infuses the context passed to Validate/SetDefaults with custom metadata.
		ctxFunc,

		// Whether to disallow unknown fields.
		true,

		// Extra validating callbacks to be applied to resources.
		callbacks,
	)
}

func main() {
	// Set up a signal context with our webhook options
	ctx := webhook.WithOptions(signals.NewContext(), webhook.Options{
		ServiceName: webhook.NameFromEnv(),
		Port:        webhook.PortFromEnv(8443),
		// SecretName must match the name of the Secret created in the configuration.
		SecretName: "eventmesh-webhook-certs",
	})

	sharedmain.WebhookMainWithContext(ctx, webhook.NameFromEnv(),
		certificates.NewController,
		NewValidationAdmissionController,
		NewDefaultingAdmissionController,
	)
}
