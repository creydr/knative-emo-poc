install:
	KO_DOCKER_REPO=localhost:5001 ko apply -Rf config/core
.PHONY: install

uninstall:
	KO_DOCKER_REPO=localhost:5001 ko resolve -Rf config/core | kubectl delete -f -
.PHONY: uninstall

clean-install: delete-kind-cluster setup-kind install
.PHONY:clean-install

setup-kind: delete-kind-cluster
	./hack/create-kind-cluster.sh
.PHONY: setup-kind

delete-kind-cluster:
	kind delete cluster
.PHONY: delete-kind-cluster