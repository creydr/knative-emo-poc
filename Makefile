install:
	KO_DOCKER_REPO=localhost:5001 ko apply -Rf config/core
.PHONY: install

uninstall:
	KO_DOCKER_REPO=localhost:5001 ko resolve -Rf config/core | kubectl delete -f -
.PHONY: uninstall

clean-install: delete-kind-cluster setup-kind install-kafka install
.PHONY:clean-install

setup-kind: delete-kind-cluster
	./hack/create-kind-cluster.sh
.PHONY: setup-kind

delete-kind-cluster:
	kind delete cluster
.PHONY: delete-kind-cluster

install-cert-manager:
	kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.18.2/cert-manager.yaml
.PHONY: install-cert-manager

install-kafka:
	kubectl create namespace kafka || true
	kubectl -n kafka apply -f 'https://strimzi.io/install/latest?namespace=kafka'
	kubectl -n kafka wait --timeout=120s --for=condition=Available deployment/strimzi-cluster-operator
	kubectl -n kafka apply -f https://strimzi.io/examples/latest/kafka/kafka-single-node.yaml
	kubectl -n kafka wait --timeout=300s --for=condition=Ready kafka/my-cluster
.PHONY: install-kafka