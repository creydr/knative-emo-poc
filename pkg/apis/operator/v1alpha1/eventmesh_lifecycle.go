package v1alpha1

import (
	"strings"

	"knative.dev/pkg/apis"
)

var EventMeshCondSet = apis.NewLivingConditionSet(EventMeshConditionInstallSucceeded, EventMeshConditionDeploymentsAvailable)

const (
	// EventMeshConditionInstallSucceeded is a Condition indicating that the installation of all components
	// has been successful.
	EventMeshConditionInstallSucceeded apis.ConditionType = "InstallSucceeded"

	// EventMeshConditionDeploymentsAvailable is a Condition indicating whether the Deployments of
	// the components have come up successfully.
	EventMeshConditionDeploymentsAvailable apis.ConditionType = "DeploymentsAvailable"
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

// MarkInstallSucceeded marks the InstallSucceeded status as true.
func (em *EventMeshStatus) MarkInstallSucceeded() {
	EventMeshCondSet.Manage(em).MarkTrue(EventMeshConditionInstallSucceeded)
}

// MarkInstallFailed marks the InstallationSucceeded status as false with the given
// message.
func (em *EventMeshStatus) MarkInstallFailed(reason, messageFormat string, messageA ...interface{}) {
	EventMeshCondSet.Manage(em).MarkFalse(EventMeshConditionInstallSucceeded, reason, messageFormat, messageA...)
}

// MarkDeploymentsAvailable marks the DeploymentsAvailable status as true.
func (em *EventMeshStatus) MarkDeploymentsAvailable() {
	EventMeshCondSet.Manage(em).MarkTrue(EventMeshConditionDeploymentsAvailable)
}

// MarkDeploymentsNotReady marks the DeploymentsAvailable status as false and calls out
// it's waiting for deployments.
func (em *EventMeshStatus) MarkDeploymentsNotReady(deployments []string) {
	EventMeshCondSet.Manage(em).MarkFalse(
		EventMeshConditionDeploymentsAvailable,
		"NotReady",
		"Waiting on deployments: %s", strings.Join(deployments, ", "))
}
