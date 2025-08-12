package scaler

import (
	"context"
	"sync"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing/pkg/apis/eventing"
	v1 "knative.dev/eventing/pkg/apis/eventing/v1"
	eventingfake "knative.dev/eventing/pkg/client/clientset/versioned/fake"
	eventinginformers "knative.dev/eventing/pkg/client/informers/externalversions"
	eventingv1listers "knative.dev/eventing/pkg/client/listers/eventing/v1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestHandleOnlyOnScaleToZeroOrOneItemsHandler(t *testing.T) {
	tests := []struct {
		name           string
		initialObjects []*v1.Broker
		addObjects     []*v1.Broker
		deleteObjects  []*v1.Broker
		expectResync   int
		description    string
	}{
		{
			name:           "add first broker triggers resync (0->1)",
			initialObjects: []*v1.Broker{},
			addObjects: []*v1.Broker{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "broker-1",
						Namespace: "default",
						Annotations: map[string]string{
							v1.BrokerClassAnnotationKey: eventing.MTChannelBrokerClassValue,
						},
					},
				},
			},
			expectResync: 1,
			description:  "Adding the first broker should trigger resync for scale-up",
		},
		{
			name: "add second broker does not trigger resync (1->2)",
			initialObjects: []*v1.Broker{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "existing-broker",
						Namespace: "default",
						Annotations: map[string]string{
							v1.BrokerClassAnnotationKey: eventing.MTChannelBrokerClassValue,
						},
					},
				},
			},
			addObjects: []*v1.Broker{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "broker-2",
						Namespace: "default",
						Annotations: map[string]string{
							v1.BrokerClassAnnotationKey: eventing.MTChannelBrokerClassValue,
						},
					},
				},
			},
			expectResync: 0,
			description:  "Adding a second broker should not trigger resync as we already have objects",
		},
		{
			name: "delete last broker triggers resync (1->0)",
			initialObjects: []*v1.Broker{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "broker-to-delete",
						Namespace: "default",
						Annotations: map[string]string{
							v1.BrokerClassAnnotationKey: eventing.MTChannelBrokerClassValue,
						},
					},
				},
			},
			deleteObjects: []*v1.Broker{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "broker-to-delete",
						Namespace: "default",
						Annotations: map[string]string{
							v1.BrokerClassAnnotationKey: eventing.MTChannelBrokerClassValue,
						},
					},
				},
			},
			expectResync: 1,
			description:  "Deleting the last broker should trigger resync for scale-down",
		},
		{
			name: "add different broker class triggers resync for new class (0->1 for Kafka)",
			initialObjects: []*v1.Broker{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mt-broker",
						Namespace: "default",
						Annotations: map[string]string{
							v1.BrokerClassAnnotationKey: eventing.MTChannelBrokerClassValue,
						},
					},
				},
			},
			addObjects: []*v1.Broker{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kafka-broker",
						Namespace: "default",
						Annotations: map[string]string{
							v1.BrokerClassAnnotationKey: "Kafka",
						},
					},
				},
			},
			expectResync: 1,
			description:  "Adding first Kafka broker should trigger resync (different class from existing MT broker)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			var resyncCalled int
			var resyncMutex sync.Mutex

			resyncFunc := func(obj interface{}) {
				resyncMutex.Lock()
				defer resyncMutex.Unlock()
				resyncCalled++
			}

			// Create fake clientset with initial objects
			var initialRuntimeObjects []runtime.Object
			for _, broker := range tt.initialObjects {
				initialRuntimeObjects = append(initialRuntimeObjects, broker)
			}
			fakeClient := eventingfake.NewSimpleClientset(initialRuntimeObjects...)

			// Create informer factory
			informerFactory := eventinginformers.NewSharedInformerFactory(fakeClient, 0)
			brokerInformer := informerFactory.Eventing().V1().Brokers()

			// Create fake informer that implements the dynamicinformer.Informer interface
			fakeBrokerInformer := &fakeInformerWrapper[eventingv1listers.BrokerLister]{
				informer: brokerInformer.Informer(),
				lister:   brokerInformer.Lister(),
			}

			// Create the handler using the actual function
			handler := handleOnlyOnScaleToZeroOrOneItemsHandler[v1.Broker, eventingv1listers.BrokerLister](
				resyncFunc,
				brokerClassFilter(),
			)(ctx, fakeBrokerInformer)

			// Start informers and wait for cache sync
			informerFactory.Start(ctx.Done())
			informerFactory.WaitForCacheSync(ctx.Done())

			// Handle add operations
			for _, broker := range tt.addObjects {
				// Add to fake clientset
				_, err := fakeClient.EventingV1().Brokers(broker.Namespace).Create(ctx, broker, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("Failed to create broker: %v", err)
				}
				// Add to informer cache
				err = brokerInformer.Informer().GetStore().Add(broker)
				if err != nil {
					t.Fatalf("Failed to add broker to informer cache: %v", err)
				}
				handler.OnAdd(broker, false)
			}

			// Handle delete operations
			for _, broker := range tt.deleteObjects {
				// Remove from fake clientset
				err := fakeClient.EventingV1().Brokers(broker.Namespace).Delete(ctx, broker.Name, metav1.DeleteOptions{})
				if err != nil {
					t.Fatalf("Failed to delete broker: %v", err)
				}
				// Remove from informer cache
				err = brokerInformer.Informer().GetStore().Delete(broker)
				if err != nil {
					t.Fatalf("Failed to delete broker from informer cache: %v", err)
				}
				handler.OnDelete(broker)
			}

			// Wait a bit for async operations
			time.Sleep(10 * time.Millisecond)

			// Check if resync was called as expected
			resyncMutex.Lock()
			actualResyncCalled := resyncCalled
			resyncMutex.Unlock()

			if actualResyncCalled != tt.expectResync {
				t.Errorf("Test: %s\nDescription: %s\nExpected resync called %d times, got %d times",
					tt.name, tt.description, tt.expectResync, actualResyncCalled)
			} else {
				t.Logf("✓ PASS: %s", tt.description)
			}
		})
	}
}

func TestBrokerClassFilterFunction(t *testing.T) {
	ctx := context.Background()
	_ = ctx // Used to satisfy import requirements
	filter := brokerClassFilter()

	tests := []struct {
		name           string
		changedBroker  *v1.Broker
		existingBroker *v1.Broker
		expectMatch    bool
		description    string
	}{
		{
			name: "same MT broker class should match",
			changedBroker: &v1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mt-broker-new",
					Annotations: map[string]string{
						v1.BrokerClassAnnotationKey: eventing.MTChannelBrokerClassValue,
					},
				},
			},
			existingBroker: &v1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mt-broker-existing",
					Annotations: map[string]string{
						v1.BrokerClassAnnotationKey: eventing.MTChannelBrokerClassValue,
					},
				},
			},
			expectMatch: true,
			description: "MT brokers should be grouped together for scaling decisions",
		},
		{
			name: "different broker classes should not match",
			changedBroker: &v1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mt-broker",
					Annotations: map[string]string{
						v1.BrokerClassAnnotationKey: eventing.MTChannelBrokerClassValue,
					},
				},
			},
			existingBroker: &v1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kafka-broker",
					Annotations: map[string]string{
						v1.BrokerClassAnnotationKey: "Kafka",
					},
				},
			},
			expectMatch: false,
			description: "MT and Kafka brokers should be scaled independently",
		},
		{
			name: "same Kafka broker class should match",
			changedBroker: &v1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kafka-broker-new",
					Annotations: map[string]string{
						v1.BrokerClassAnnotationKey: "Kafka",
					},
				},
			},
			existingBroker: &v1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "kafka-broker-existing",
					Annotations: map[string]string{
						v1.BrokerClassAnnotationKey: "Kafka",
					},
				},
			},
			expectMatch: true,
			description: "Kafka brokers should be grouped together for scaling decisions",
		},
		{
			name: "missing annotation on existing broker should not match",
			changedBroker: &v1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mt-broker",
					Annotations: map[string]string{
						v1.BrokerClassAnnotationKey: eventing.MTChannelBrokerClassValue,
					},
				},
			},
			existingBroker: &v1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "no-class-broker",
					Annotations: map[string]string{},
				},
			},
			expectMatch: false,
			description: "Brokers without broker class annotations should not match",
		},
		{
			name: "missing annotation on changed broker should not match",
			changedBroker: &v1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "no-class-broker",
					Annotations: map[string]string{},
				},
			},
			existingBroker: &v1.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "mt-broker",
					Annotations: map[string]string{
						v1.BrokerClassAnnotationKey: eventing.MTChannelBrokerClassValue,
					},
				},
			},
			expectMatch: false,
			description: "Brokers without broker class annotations should not match",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter(tt.changedBroker, tt.existingBroker)
			if result != tt.expectMatch {
				t.Errorf("Test: %s\nDescription: %s\nExpected match=%v, got match=%v",
					tt.name, tt.description, tt.expectMatch, result)
			} else {
				t.Logf("✓ PASS: %s", tt.description)
			}
		})
	}
}

// fakeInformerWrapper wraps a informer to implement dynamicinformer.Informer interface
type fakeInformerWrapper[L any] struct {
	informer cache.SharedIndexInformer
	lister   L
}

func (f *fakeInformerWrapper[L]) Informer() cache.SharedIndexInformer {
	return f.informer
}

func (f *fakeInformerWrapper[L]) Lister() L {
	return f.lister
}
