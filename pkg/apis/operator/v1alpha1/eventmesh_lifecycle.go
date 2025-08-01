package v1alpha1

import "knative.dev/pkg/apis"

var EventMeshCondSet = apis.NewLivingConditionSet(EventMeshConditionEventingInstalled, EventMeshConditionEventingKafkaBrokerInstalled)

const (
	EventMeshConditionReady                                           = apis.ConditionReady
	EventMeshConditionEventingInstalled            apis.ConditionType = "EventingInstalled"
	EventMeshConditionEventingKafkaBrokerInstalled apis.ConditionType = "EventingKafkaBrokerInstalled"
	EventMeshConditionEventingConfigured           apis.ConditionType = "EventingConfigured"
)

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*EventMesh) GetConditionSet() apis.ConditionSet {
	return EventMeshCondSet
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (em *EventMeshStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return EventMeshCondSet.Manage(em).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (em *EventMeshStatus) IsReady() bool {
	return em.GetTopLevelCondition().IsTrue()
}

// GetTopLevelCondition returns the top level Condition.
func (em *EventMeshStatus) GetTopLevelCondition() *apis.Condition {
	return EventMeshCondSet.Manage(em).GetTopLevelCondition()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (em *EventMeshStatus) InitializeConditions() {
	EventMeshCondSet.Manage(em).InitializeConditions()
}

func (em *EventMeshStatus) MarkEventMeshConditionEventingInstalledEnabled() {
	EventMeshCondSet.Manage(em).MarkTrue(EventMeshConditionEventingInstalled)
}

func (em *EventMeshStatus) MarkEventMeshConditionEventingInstalledDisabled(reason, messageFormat string, messageA ...interface{}) {
	EventMeshCondSet.Manage(em).MarkFalse(EventMeshConditionEventingInstalled, reason, messageFormat, messageA...)
}

func (em *EventMeshStatus) MarkEventMeshConditionEventingConfiguredEnabled() {
	EventMeshCondSet.Manage(em).MarkTrue(EventMeshConditionEventingConfigured)
}

func (em *EventMeshStatus) MarkEventMeshConditionEventingConfiguredDisabled(reason, messageFormat string, messageA ...interface{}) {
	EventMeshCondSet.Manage(em).MarkFalse(EventMeshConditionEventingConfigured, reason, messageFormat, messageA...)
}

func (em *EventMeshStatus) MarkEventMeshConditionEventingKafkaBrokerInstalledEnabled() {
	EventMeshCondSet.Manage(em).MarkTrue(EventMeshConditionEventingKafkaBrokerInstalled)
}

func (em *EventMeshStatus) MarkEventMeshConditionEventingKafkaBrokerInstalledDisabled(reason, messageFormat string, messageA ...interface{}) {
	EventMeshCondSet.Manage(em).MarkFalse(EventMeshConditionEventingKafkaBrokerInstalled, reason, messageFormat, messageA...)
}
