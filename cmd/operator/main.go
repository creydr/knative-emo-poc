package main

import (
	"github.com/creydr/knative-emo-poc/pkg/reconciler/eventmesh"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
)

func main() {
	ctx := signals.NewContext()

	sharedmain.MainWithContext(ctx, "eventmesh-operator",
		eventmesh.NewController,
	)
}
