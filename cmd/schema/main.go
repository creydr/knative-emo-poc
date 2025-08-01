package main

import (
	"log"

	"knative.dev/hack/schema/commands"
	"knative.dev/hack/schema/registry"

	operatorv1alpha1 "github.com/creydr/knative-emo-poc/pkg/apis/operator/v1alpha1"
)

// schema is a tool to dump the schema for Eventing resources.
func main() {
	// Eventing
	registry.Register(&operatorv1alpha1.EventMesh{})

	if err := commands.New("github.com/creydr/knative-emo-poc").Execute(); err != nil {
		log.Fatal("Error during command execution: ", err)
	}
}
