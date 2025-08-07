package main

import (
	"log"

	"knative.dev/hack/schema/commands"
	"knative.dev/hack/schema/registry"

	operatorv1alpha1 "knative.dev/eventmesh-operator/pkg/apis/operator/v1alpha1"
)

// schema is a tool to dump the schema for Eventing resources.
func main() {
	// Eventing
	registry.Register(&operatorv1alpha1.EventMesh{})

	if err := commands.New("knative.dev/eventmesh-operator").Execute(); err != nil {
		log.Fatal("Error during command execution: ", err)
	}
}
