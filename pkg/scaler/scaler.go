package scaler

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/eventing"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	messagingv1 "knative.dev/eventing/pkg/apis/messaging/v1"
	eventinginformers "knative.dev/eventing/pkg/client/informers/externalversions"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	eventingv1listers "knative.dev/eventing/pkg/client/listers/eventing/v1"
	messagingv1listers "knative.dev/eventing/pkg/client/listers/messaging/v1"
	"knative.dev/eventmesh-operator/pkg/dynamicinformer"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const (
	brokerCRDName = "brokers.eventing.knative.dev"
	imcCRDName    = "inmemorychannels.messaging.knative.dev"
)

type Scaler struct {
	logger *zap.SugaredLogger

	dynamicIMCInformer   *dynamicinformer.DynamicInformer[messagingv1.InMemoryChannel, messagingv1listers.InMemoryChannelLister]
	dynamicBrokerInfomer *dynamicinformer.DynamicInformer[v1.Broker, eventingv1listers.BrokerLister]
}

func New(ctx context.Context) *Scaler {
	logger := logging.FromContext(ctx).With(zap.String("component", "scaler"))

	dynamicIMCInformer := dynamicinformer.New(imcCRDName, func(ctx context.Context) (dynamicinformer.SharedInformerFactory, dynamicinformer.Informer[messagingv1listers.InMemoryChannelLister]) {
		factory := eventinginformers.NewSharedInformerFactory(eventingclient.Get(ctx), controller.GetResyncPeriod(ctx))

		return factory, factory.Messaging().V1().InMemoryChannels()
	})

	dynamicBrokerInformer := dynamicinformer.New(brokerCRDName, func(ctx context.Context) (dynamicinformer.SharedInformerFactory, dynamicinformer.Informer[eventingv1listers.BrokerLister]) {
		factory := eventinginformers.NewSharedInformerFactory(eventingclient.Get(ctx), controller.GetResyncPeriod(ctx))

		return factory, factory.Eventing().V1().Brokers()
	})

	return &Scaler{
		logger:               logger,
		dynamicIMCInformer:   dynamicIMCInformer,
		dynamicBrokerInfomer: dynamicBrokerInformer,
	}
}

func (s *Scaler) IMCScaleTarget() (int, error) {
	var imcScaleTarget int

	imcLister := s.dynamicIMCInformer.Lister().Load()

	if imcLister == nil || *imcLister == nil {
		// no imc lister registered so far (probably because IMC CRD is installed yet)
		imcScaleTarget = 0
	} else {
		imcs, err := (*imcLister).List(labels.Everything())
		if err != nil {
			return 0, fmt.Errorf("failed to list all InMemoryChannels: %w", err)
		}

		s.logger.Debugf("Found %d in-memory channels", len(imcs))

		if len(imcs) == 0 {
			imcScaleTarget = 0
		} else {
			imcScaleTarget = 1
		}
	}

	return imcScaleTarget, nil
}

func (s *Scaler) MTBrokerScaleTarget() (int, error) {
	var mtBrokerScaleTarget int
	brokerLister := s.dynamicBrokerInfomer.Lister().Load()

	if brokerLister == nil || *brokerLister == nil {
		// no broker lister registered so far (probably because Broker CRD is installed yet)
		mtBrokerScaleTarget = 0
	} else {
		brokers, err := (*brokerLister).List(labels.Everything())
		if err != nil {
			return 0, fmt.Errorf("failed to list all Brokers: %w", err)
		}

		mtBrokers := make([]*v1.Broker, 0, len(brokers))
		for _, broker := range brokers {
			if broker.Annotations[v1.BrokerClassAnnotationKey] == eventing.MTChannelBrokerClassValue {
				mtBrokers = append(mtBrokers, broker)
			}
		}

		s.logger.Debugf("Found %d mt brokers (%d brokers in total)", len(mtBrokers), len(brokers))

		if len(mtBrokers) == 0 {
			mtBrokerScaleTarget = 0
		} else {
			mtBrokerScaleTarget = 1
		}
	}

	return mtBrokerScaleTarget, nil
}

func (s *Scaler) KafkaBrokerScaleTarget() int {
	panic("implement me")
	return 0
}

func (s *Scaler) CRDEventHandler(ctx context.Context, globalResync func(interface{})) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			s.logger.Debug("OnAdd in CRDEventHandler")

			crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
			if !ok {
				s.logger.Warn("Received unexpected object, ignoring add")
				return
			}

			// setup dynamic informer depending on CRD
			var err error
			switch crd.Name {
			case brokerCRDName:
				err = s.dynamicBrokerInfomer.SetupInformerAndRegisterEventHandler(ctx, handleOnlyOnScaleToZeroOrOneItemsHandler[v1.Broker, eventingv1listers.BrokerLister](globalResync, brokerClassFilter()))
			case imcCRDName:
				err = s.dynamicIMCInformer.SetupInformerAndRegisterEventHandler(ctx, handleOnlyOnScaleToZeroOrOneItemsHandler[messagingv1.InMemoryChannel, messagingv1listers.InMemoryChannelLister](globalResync, nil))
			default:
				// unrelated CRD
			}

			if err != nil {
				logging.FromContext(ctx).Errorw("Failed to register dynamic informer for CRD", zap.String("crd", crd.Name), zap.Error(err))
			}
		},
		UpdateFunc: func(_, _ interface{}) {}, // ignore updates (we care only if the CRD was created or removed)
		DeleteFunc: func(obj interface{}) {
			s.logger.Debug("OnDelete in CRDEventHandler")

			crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
			if !ok {
				s.logger.Warn("Received unexpected object, ignoring delete")
				return
			}

			logging.FromContext(ctx).Debug("CRD is removed, stopping informer", zap.String("crd", crd.Name))

			// stop dynamic informer depending on CRD
			switch crd.Name {
			case brokerCRDName:
				s.dynamicBrokerInfomer.Stop(ctx)
			case imcCRDName:
				s.dynamicIMCInformer.Stop(ctx)
			default:
				// unrelated CRD
			}
		},
	}
}

// handleOnlyOnScaleToZeroOrOneItemsHandler only calls the resyncFunc, when the first resource gets created or
// when the last resource gets deleted. This is helpful to trigger the resyncFunc only on meaningful updates for the
// scaling (so not every time a resource (e.g. a Channel) gets created the whole EventMesh gets reconciled).
// It also accepts an optional filter, which lets filter the objects. This can be helpful for example, when a resource
// is handled by different controllers which need to scale up and down separately (for example the brokers where we
// have the kafka or mt-channel broker controllers)
// Simply said, it is an advanced FilteringResourceEventHandler.
func handleOnlyOnScaleToZeroOrOneItemsHandler[T any, Lister dynamicinformer.SimpleLister[T]](resyncFunc func(obj interface{}), filterFunc func(changedObj *T, existingObj *T) bool) dynamicinformer.EventHandlerFunc[T, Lister] {
	return func(ctx context.Context, informer dynamicinformer.Informer[Lister]) cache.ResourceEventHandler {
		logger := logging.FromContext(ctx)
		return cache.ResourceEventHandlerFuncs{
			AddFunc: func(newObj interface{}) {
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
							if filterFunc(newObj.(*T), obj) {
								filteredObjs = append(filteredObjs, obj)
							}
						}
					}

					logger.Debugf("OnAdd with %d objects existing now (%d in total before filtering)", len(filteredObjs), len(objs))

					if len(filteredObjs) == 1 {
						// we only care, when the first object comes
						resyncFunc(newObj)
					} else {
						logger.Debug("Skipping triggering a reconcile, as no meaningful change in number of occurrences")
					}
				}
			},
			UpdateFunc: func(old, new interface{}) {
				// updates are ignored
			},
			DeleteFunc: func(deletedObj interface{}) {
				objs, err := informer.Lister().List(labels.Everything())
				if err != nil {
					logger.Warn("Failed to list informer resources", zap.Error(err))
					return
				}

				filteredObjs := objs
				if filterFunc != nil {
					filteredObjs = make([]*T, 0, len(objs))
					for _, obj := range objs {
						if filterFunc(deletedObj.(*T), obj) {
							filteredObjs = append(filteredObjs, obj)
						}
					}
				}

				logger.Debugf("OnDelete with %d objects existing now (%d in total before filtering)", len(filteredObjs), len(objs))

				if len(filteredObjs) == 0 {
					// we only care, when the last one gets removed...
					resyncFunc(deletedObj)
				} else {
					logger.Debug("Skipping triggering a reconcile, as no meaningful change in number of occurrences")
				}
			},
		}
	}
}

// brokerClassFilter returns a filter, which lets only objects pass, which have the same broker class as the changed broker
func brokerClassFilter() func(*v1.Broker, *v1.Broker) bool {
	return func(changedBroker, foundBroker *v1.Broker) bool {
		return changedBroker.Annotations[v1.BrokerClassAnnotationKey] == foundBroker.Annotations[v1.BrokerClassAnnotationKey]
	}
}
