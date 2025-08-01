#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source $(dirname $0)/../vendor/knative.dev/hack/library.sh

while IFS= read -r nightly; do

  target_dir=$(echo $nightly | jq -r '.directory')
  bucket=$(echo $nightly | jq -r '.bucket')

  rm -rf "${REPO_ROOT_DIR}/${target_dir}"
  mkdir -p "${REPO_ROOT_DIR}/${target_dir}"

  for file in $(echo $nightly | jq -r '.files[]'); do
    curl -s https://storage.googleapis.com/knative-nightly/${bucket}/latest/${file} \
      --create-dirs \
      -o "${REPO_ROOT_DIR}/${target_dir}/${file}"
  done

done <<< "$(yq read --tojson "${REPO_ROOT_DIR}/hack/nightlies.yaml" '[*]')"