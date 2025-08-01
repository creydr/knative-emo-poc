#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source $(dirname "$0")/../vendor/knative.dev/hack/library.sh

go_update_deps "$@"

${REPO_ROOT_DIR}/hack/update-nightlies.sh