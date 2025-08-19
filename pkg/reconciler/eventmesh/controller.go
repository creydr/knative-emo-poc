package eventmesh

import (
	"context"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"go.uber.org/zap"
	eventmeshinformer "knative.dev/eventmesh-operator/pkg/client/injection/informers/operator/v1alpha1/eventmesh"
	eventmeshreconciler "knative.dev/eventmesh-operator/pkg/client/injection/reconciler/operator/v1alpha1/eventmesh"
	"knative.dev/eventmesh-operator/pkg/manifests"
	"knative.dev/eventmesh-operator/pkg/scaler"
	crdinformer "knative.dev/pkg/client/injection/apiextensions/informers/apiextensions/v1/customresourcedefinition"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	eventMeshInformer := eventmeshinformer.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)
	crdInformer := crdinformer.Get(ctx)
	scaler := scaler.New(ctx)

	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		logger.Fatalw("Error creating client from injected config", zap.Error(err))
	}
	mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
	manifest, _ := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))

	eventingParser := manifests.NewEventingParser()
	kafkaBrokerParser := manifests.NewKafkaBrokerParser()

	r := &Reconciler{
		eventMeshLister:   eventMeshInformer.Lister(),
		deploymentLister:  deploymentInformer.Lister(),
		crdLister:         crdInformer.Lister(),
		manifest:          manifest,
		scaler:            scaler,
		eventingParser:    eventingParser,
		kafkaBrokerParser: kafkaBrokerParser,
	}

	impl := eventmeshreconciler.NewImpl(ctx, r)
	globalResync := func(interface{}) {
		impl.GlobalResync(eventMeshInformer.Informer())
	}

	eventMeshInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	crdInformer.Informer().AddEventHandler(scaler.CRDEventHandler(ctx, globalResync))

	return impl
}
