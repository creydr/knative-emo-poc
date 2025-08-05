package v1alpha1

import "knative.dev/pkg/apis"

var EventMeshCondSet = apis.NewLivingConditionSet(EventMeshConditionEventMeshInstalled)

const (
	EventMeshConditionReady                                 = apis.ConditionReady
	EventMeshConditionEventMeshInstalled apis.ConditionType = "EventingInstalled"
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

func (em *EventMeshStatus) MarkEventMeshConditionEventMeshInstalled() {
	EventMeshCondSet.Manage(em).MarkTrue(EventMeshConditionEventMeshInstalled)
}

func (em *EventMeshStatus) MarkEventMeshConditionEventMeshInstalledFalse(reason, messageFormat string, messageA ...interface{}) {
	EventMeshCondSet.Manage(em).MarkFalse(EventMeshConditionEventMeshInstalled, reason, messageFormat, messageA...)
}
