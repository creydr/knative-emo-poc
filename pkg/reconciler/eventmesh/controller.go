package eventmesh

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	eventmeshinformer "github.com/creydr/knative-emo-poc/pkg/client/injection/informers/operator/v1alpha1/eventmesh"
	eventmeshreconciler "github.com/creydr/knative-emo-poc/pkg/client/injection/reconciler/operator/v1alpha1/eventmesh"
	"github.com/go-logr/zapr"
	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	eventinginformers "knative.dev/eventing/pkg/client/informers/externalversions"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	messagingv1listers "knative.dev/eventing/pkg/client/listers/messaging/v1"
	"knative.dev/pkg/client/injection/apiextensions/client"
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
	imcInformer := NewDynamicIMCInformer()

	mfclient, err := mfc.NewClient(injection.GetConfig(ctx))
	if err != nil {
		logger.Fatalw("Error creating client from injected config", zap.Error(err))
	}
	mflogger := zapr.NewLogger(logger.Named("manifestival").Desugar())
	manifest, _ := mf.ManifestFrom(mf.Slice{}, mf.UseClient(mfclient), mf.UseLogger(mflogger))

	r := &Reconciler{
		eventMeshLister:  eventMeshInformer.Lister(),
		deploymentLister: deploymentInformer.Lister(),
		manifest:         manifest,
		imcLister:        imcInformer.Lister(),
	}

	impl := eventmeshreconciler.NewImpl(ctx, r)

	eventMeshInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	globalResync := func(interface{}) {
		impl.GlobalResync(eventMeshInformer.Informer())
	}

	crdInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithName("inmemorychannels.messaging.knative.dev"),
		Handler:    crdHandler(createIMCInformerFunc(ctx, imcInformer, globalResync), stopIMCInformerFunc(ctx, imcInformer)),
	})

	return impl
}

type DynamicIMCInformer struct {
	cancel atomic.Pointer[context.CancelFunc]
	lister atomic.Pointer[messagingv1listers.InMemoryChannelLister]
	mu     sync.Mutex
}

func NewDynamicIMCInformer() *DynamicIMCInformer {
	return &DynamicIMCInformer{
		cancel: atomic.Pointer[context.CancelFunc]{},
		lister: atomic.Pointer[messagingv1listers.InMemoryChannelLister]{},
		mu:     sync.Mutex{},
	}
}

func (df *DynamicIMCInformer) Reconcile(ctx context.Context, resyncFunc func(interface{})) error {
	logger := logging.FromContext(ctx).With(zap.String("component", "DynamicIMCInformer"))

	df.mu.Lock()
	defer df.mu.Unlock()

	if df.cancel.Load() != nil {
		logger.Debugw("Cancel function already loaded, skipping start")
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	factory := eventinginformers.NewSharedInformerFactory(eventingclient.Get(ctx), controller.GetResyncPeriod(ctx))
	informer := factory.Messaging().V1().InMemoryChannels()
	informer.Informer().AddEventHandler(controller.HandleAll(resyncFunc))

	err := wait.PollUntilContextCancel(ctx, time.Second, false, func(ctx context.Context) (done bool, err error) {
		crd := "inmemorychannels.messaging.knative.dev"
		logger.Debugf("Waiting for %s CRD to be installed", crd)
		isCRDInstalled, err := df.isCRDInstalled(ctx, crd)
		if err != nil {
			return false, nil
		}
		return isCRDInstalled, nil
	})
	if err != nil {
		defer cancel()
		return fmt.Errorf("could not check if IMC CRD is installed: %w", err)
	}

	factory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
		defer cancel()
		logger.Errorw("Failed to sync imc informer cache")
		return fmt.Errorf("failed to sync imc informer")
	}

	lister := informer.Lister()
	df.lister.Store(&lister)
	df.cancel.Store(&cancel) // Cancel is always set as last field since it's used as a "guard".

	return nil
}

func (df *DynamicIMCInformer) isCRDInstalled(ctx context.Context, name string) (bool, error) {
	apiExtensionsClient := client.Get(ctx)

	_, err := apiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get %s CRD: %w", name, err)
	}
	return true, nil
}

func (df *DynamicIMCInformer) stop(ctx context.Context) {
	cancel := df.cancel.Load()
	if cancel == nil {
		logging.FromContext(ctx).Debugw("IMC informer has not been started, nothing to stop")
		return
	}

	(*cancel)()
	df.lister.Store(nil)
	df.cancel.Store(nil) // Cancel is always set as last field since it's used as a "guard".
}

func (df *DynamicIMCInformer) Lister() *atomic.Pointer[messagingv1listers.InMemoryChannelLister] {
	df.mu.Lock()
	defer df.mu.Unlock()

	return &df.lister
}

func createIMCInformerFunc(ctx context.Context, df *DynamicIMCInformer, resyncFunc func(interface{})) func() {
	return func() {
		if err := df.Reconcile(ctx, resyncFunc); err != nil {
			logging.FromContext(ctx).Errorf("Failed reconciling informer for DynamicIMC: %v", err)
		}
	}
}

func stopIMCInformerFunc(ctx context.Context, df *DynamicIMCInformer) func() {
	return func() {
		logging.FromContext(ctx).Debug("IMC CRD is removed, stopping IMC informer")
		df.stop(ctx)
	}
}

func crdHandler(onAdd, onDelete func()) cache.ResourceEventHandler {
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
