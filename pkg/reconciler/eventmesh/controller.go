package eventmesh

import (
	"context"

	"github.com/creydr/knative-emo-poc/pkg/client/injection/informers/operator/v1alpha1/eventmesh"
	eventmeshreconciler "github.com/creydr/knative-emo-poc/pkg/client/injection/reconciler/operator/v1alpha1/eventmesh"
	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"go.uber.org/zap"
	imcinformer "knative.dev/eventing/pkg/client/injection/informers/messaging/v1/inmemorychannel"
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

	eventMeshInformer := eventmesh.Get(ctx)
	inmemorychannelInformer := imcinformer.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)

	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		logger.Fatalw("Error creating client from injected config", zap.Error(err))
	}
	mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
	manifest, _ := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))

	r := &Reconciler{
		eventMeshLister:       eventMeshInformer.Lister(),
		inMemoryChannelLister: inmemorychannelInformer.Lister(),
		deploymentLister:      deploymentInformer.Lister(),
		manifest:              manifest,
	}

	impl := eventmeshreconciler.NewImpl(ctx, r)

	eventMeshInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	globalResync := func(interface{}) {
		impl.GlobalResync(eventMeshInformer.Informer())
	}

	inmemorychannelInformer.Informer().AddEventHandler(controller.HandleAll(globalResync))

	return impl
}
