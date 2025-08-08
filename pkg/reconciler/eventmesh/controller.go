package eventmesh

import (
	"context"

	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/eventing"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventinginformers "knative.dev/eventing/pkg/client/informers/externalversions"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	eventingv1listers "knative.dev/eventing/pkg/client/listers/eventing/v1"
	messagingv1listers "knative.dev/eventing/pkg/client/listers/messaging/v1"
	eventmeshinformer "knative.dev/eventmesh-operator/pkg/client/injection/informers/operator/v1alpha1/eventmesh"
	eventmeshreconciler "knative.dev/eventmesh-operator/pkg/client/injection/reconciler/operator/v1alpha1/eventmesh"
	"knative.dev/eventmesh-operator/pkg/dynamicinformer"
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

	imcCrdName := "inmemorychannels.messaging.knative.dev"
	dynamicIMCInformer := dynamicinformer.New(imcCrdName, func(ctx context.Context) (dynamicinformer.SharedInformerFactory, dynamicinformer.Informer[messagingv1listers.InMemoryChannelLister]) {
		factory := eventinginformers.NewSharedInformerFactory(eventingclient.Get(ctx), controller.GetResyncPeriod(ctx))

		return factory, factory.Messaging().V1().InMemoryChannels()
	})

	brokerCrdName := "brokers.eventing.knative.dev"
	dynamicMTBrokerInformer := dynamicinformer.New(brokerCrdName, func(ctx context.Context) (dynamicinformer.SharedInformerFactory, dynamicinformer.Informer[eventingv1listers.BrokerLister]) {
		factory := eventinginformers.NewSharedInformerFactory(eventingclient.Get(ctx), controller.GetResyncPeriod(ctx))

		return factory, factory.Eventing().V1().Brokers()
	})

	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		logger.Fatalw("Error creating client from injected config", zap.Error(err))
	}
	mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
	manifest, _ := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))

	r := &Reconciler{
		eventMeshLister:       eventMeshInformer.Lister(),
		deploymentLister:      deploymentInformer.Lister(),
		manifest:              manifest,
		dynamicIMCLister:      dynamicIMCInformer.Lister(),
		dynamicMTBrokerLister: dynamicMTBrokerInformer.Lister(),
	}

	impl := eventmeshreconciler.NewImpl(ctx, r)

	eventMeshInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	globalResync := func(interface{}) {
		impl.GlobalResync(eventMeshInformer.Informer())
	}

	crdInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithName(imcCrdName),
		Handler:    newCRDEventHandler(startDynamicInformer(ctx, dynamicIMCInformer, globalResync, nil), stopDynamicInformer(ctx, dynamicIMCInformer)),
	})

	crdInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithName(brokerCrdName),
		Handler:    newCRDEventHandler(startDynamicInformer(ctx, dynamicMTBrokerInformer, globalResync, brokerClassFilter(eventing.MTChannelBrokerClassValue)), stopDynamicInformer(ctx, dynamicMTBrokerInformer)),
	})

	return impl
}

func brokerClassFilter(brokerClass string) func(*v1.Broker) bool {
	return func(broker *v1.Broker) bool {
		return broker.Annotations[v1.BrokerClassAnnotationKey] == brokerClass
	}
}

func startDynamicInformer[T any, Lister dynamicinformer.SimpleLister[T]](ctx context.Context, di *dynamicinformer.DynamicInformer[T, Lister], resyncFunc func(interface{}), filterFunc func(obj *T) bool) func() {
	return func() {
		if err := di.SetupInformerAndRegisterEventHandler(ctx, handleOnlyOnScaleToZeroOrOneItemsHandler[T, Lister](ctx, resyncFunc, filterFunc)); err != nil {
			logging.FromContext(ctx).Errorf("Failed reconciling dynamic informer: %v", err)
		}
	}
}

func stopDynamicInformer[T any, Lister dynamicinformer.SimpleLister[T]](ctx context.Context, di *dynamicinformer.DynamicInformer[T, Lister]) func() {
	return func() {
		logging.FromContext(ctx).Debug("CRD is removed, stopping informer")
		di.Stop(ctx)
	}
}

func newCRDEventHandler(onAdd, onDelete func()) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			onAdd()
		},
		UpdateFunc: func(_, _ interface{}) {}, // ignore updates (we care only if the CRD was created or removed)
		DeleteFunc: func(_ interface{}) {
			onDelete()
		},
	}
}

// handleOnlyOnScaleToZeroOrOneItemsHandler only calls the resyncFunc, when the first resource gets created or
// when the last resource gets deleted. This is helpful to trigger the resyncFunc only on meaningful updates for the
// scaling (so not every time a resource (e.g. a Channel) gets created the whole EventMesh gets reconciled).
// It also accepts an optional filter, which lets filter the objects. This can be helpful for example, when a resource
// is handled by different controllers which need to scale up and down (for example the brokers where we have the kafka
// or mt-channel broker controllers)
// Simply said, it is an advanced FilteringResourceEventHandler.
func handleOnlyOnScaleToZeroOrOneItemsHandler[T any, Lister dynamicinformer.SimpleLister[T]](ctx context.Context, resyncFunc func(obj interface{}), filterFunc func(*T) bool) dynamicinformer.EventHandlerFunc[T, Lister] {
	return func(informer dynamicinformer.Informer[Lister]) cache.ResourceEventHandler {
		logger := logging.FromContext(ctx)
		return cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if informer.Informer() != nil && informer.Informer().HasSynced() {
					objs, err := informer.Lister().List(labels.Everything())
					if err != nil {
						logger.Warn("Failed to list informer resources", zap.Error(err))
						return
					}

					filteredObjs := objs
					if filterFunc != nil {
						filteredObjs = make([]*T, 0, len(objs))
						for _, obj := range objs {
							if filterFunc(obj) {
								filteredObjs = append(filteredObjs, obj)
							}
						}
					}

					logger.Debugf("OnAdd with %d objects existing now (%d in total before filtering)", len(filteredObjs), len(objs))

					if len(filteredObjs) == 1 {
						// we only care, when the first object comes
						resyncFunc(obj)
					}
				}
			},
			UpdateFunc: func(old, new interface{}) {
				// updates are ignored
			},
			DeleteFunc: func(obj interface{}) {
				objs, err := informer.Lister().List(labels.Everything())
				if err != nil {
					logger.Warn("Failed to list informer resources", zap.Error(err))
					return
				}

				filteredObjs := objs
				if filterFunc != nil {
					filteredObjs = make([]*T, 0, len(objs))
					for _, obj := range objs {
						if filterFunc(obj) {
							filteredObjs = append(filteredObjs, obj)
						}
					}
				}

				logger.Debugf("OnDelete with %d objects existing now (%d in total before filtering)", len(filteredObjs), len(objs))

				if len(filteredObjs) == 0 {
					// we only care, when the last one gets removed...
					resyncFunc(obj)
				}
			},
		}
	}
}
