#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source $(dirname $0)/../vendor/knative.dev/hack/codegen-library.sh
source "${CODEGEN_PKG}/kube_codegen.sh"

# If we run with -mod=vendor here, then generate-groups.sh looks for vendor files in the wrong place.
export GOFLAGS=-mod=

echo "=== Update Codegen for $MODULE_NAME"

group "Kubernetes Codegen"

kube::codegen::gen_helpers \
  --boilerplate "${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt" \
  "${REPO_ROOT_DIR}/pkg/apis"

kube::codegen::gen_client \
  --boilerplate "${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt" \
  --output-dir "${REPO_ROOT_DIR}/pkg/client" \
  --output-pkg "knative.dev/eventmesh-operator/pkg/client" \
  --with-watch \
  "${REPO_ROOT_DIR}/pkg/apis"

group "Knative Codegen"

# Knative Injection
${KNATIVE_CODEGEN_PKG}/hack/generate-knative.sh "injection" \
  knative.dev/eventmesh-operator/pkg/client knative.dev/eventmesh-operator/pkg/apis \
  "operator:v1alpha1" \
  --go-header-file ${REPO_ROOT_DIR}/hack/boilerplate/boilerplate.go.txt

group "Update CRDs"
# EventMesh CRD
yq w -i config/core/resources/eventmesh.yaml 'spec.versions.(name==v1alpha1).schema.openAPIV3Schema' "$(go run cmd/schema/main.go dump EventMesh)"
sed -i 's/ openAPIV3Schema: |-/ openAPIV3Schema:/' config/core/resources/eventmesh.yaml

group "Update deps post-codegen"

# Make sure our dependencies are up-to-date
${REPO_ROOT_DIR}/hack/update-deps.sh

# Make sure our nightly yamls are up-to-date
${REPO_ROOT_DIR}/hack/update-eventing-manifests.sh
