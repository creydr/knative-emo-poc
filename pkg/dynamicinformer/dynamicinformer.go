package dynamicinformer

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	eventinginformers "knative.dev/eventing/pkg/client/informers/externalversions"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/pkg/client/injection/apiextensions/client"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

type DynamicInformer[Lister any] struct {
	cancel      atomic.Pointer[context.CancelFunc]
	lister      atomic.Pointer[Lister]
	factoryFunc FactoryFunc[Lister]
	crdName     string
	mu          sync.Mutex
}

type FactoryFunc[Lister any] func(ctx context.Context) (SharedInformerFactory, Informer[Lister])

type Informer[Lister any] interface {
	Informer() cache.SharedIndexInformer
	Lister() Lister
}

type SharedInformerFactory interface {
	Start(stopCh <-chan struct{})
	Shutdown()
	WaitForCacheSync(stopCh <-chan struct{}) map[reflect.Type]bool
}

func New[Lister any](crdName string, factoryFunc FactoryFunc[Lister]) *DynamicInformer[Lister] {
	return &DynamicInformer[Lister]{
		cancel:      atomic.Pointer[context.CancelFunc]{},
		lister:      atomic.Pointer[Lister]{},
		mu:          sync.Mutex{},
		factoryFunc: factoryFunc,
		crdName:     crdName,
	}
}

func (di *DynamicInformer[Lister]) Reconcile(ctx context.Context, resyncFunc func(interface{})) error {
	logger := logging.FromContext(ctx).With(zap.String("component", "DynamicInformer"), zap.String("resource", di.crdName))

	di.mu.Lock()
	defer di.mu.Unlock()

	if di.cancel.Load() != nil {
		logger.Debug("Cancel function already loaded, skipping start")
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	eventinginformers.NewSharedInformerFactory(eventingclient.Get(ctx), controller.GetResyncPeriod(ctx))
	factory, informer := di.factoryFunc(ctx)

	informer.Informer().AddEventHandler(controller.HandleAll(resyncFunc)) //TODO: think about adding an event handler which only resyncs on add & update (because this is what we care about for scaling)

	err := wait.PollUntilContextCancel(ctx, time.Second, false, func(ctx context.Context) (done bool, err error) {
		logger.Debugf("Waiting for %s CRD to be installed", di.crdName)
		isCRDInstalled, err := di.isCRDInstalled(ctx)
		if err != nil {
			return false, nil
		}
		return isCRDInstalled, nil
	})
	if err != nil {
		defer cancel()
		return fmt.Errorf("could not check if CRD is installed: %w", err)
	}

	factory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.Informer().HasSynced) {
		defer cancel()
		logger.Error("Failed to sync dynamic informer cache")
		return fmt.Errorf("failed to sync dynamic informer")
	}

	lister := informer.Lister()
	di.lister.Store(&lister)
	di.cancel.Store(&cancel) // Cancel is always set as last field since it's used as a "guard".

	return nil
}

func (di *DynamicInformer[Lister]) isCRDInstalled(ctx context.Context) (bool, error) {
	apiExtensionsClient := client.Get(ctx)

	_, err := apiExtensionsClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, di.crdName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get %s CRD: %w", di.crdName, err)
	}
	return true, nil
}

func (di *DynamicInformer[Lister]) Stop(ctx context.Context) {
	cancel := di.cancel.Load()
	if cancel == nil {
		logging.FromContext(ctx).Debug("Dynamic informer has not been started, nothing to stop")
		return
	}

	(*cancel)()
	di.lister.Store(nil)
	di.cancel.Store(nil) // Cancel is always set as last field since it's used as a "guard".
}

func (di *DynamicInformer[Lister]) Lister() *atomic.Pointer[Lister] {
	di.mu.Lock()
	defer di.mu.Unlock()

	return &di.lister
}
