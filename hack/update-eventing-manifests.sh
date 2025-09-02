#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

source $(dirname $0)/../vendor/knative.dev/hack/library.sh

RELEASE_VERSION=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --release)
      RELEASE_VERSION="$2"
      shift 2
      ;;
    *)
      echo "Unknown option $1"
      exit 1
      ;;
  esac
done

while IFS= read -r nightly; do

  target_dir=$(echo $nightly | jq -r '.directory')
  bucket=$(echo $nightly | jq -r '.bucket')

  rm -rf "${REPO_ROOT_DIR}/${target_dir}"
  mkdir -p "${REPO_ROOT_DIR}/${target_dir}"

  for file in $(echo $nightly | jq -r '.files[]'); do
    if [[ -n "$RELEASE_VERSION" ]]; then
      curl -s "https://storage.googleapis.com/knative-releases/${bucket}/previous/v${RELEASE_VERSION}/${file}" \
        --create-dirs \
        -o "${REPO_ROOT_DIR}/${target_dir}/${file}"
    else
      curl -s "https://storage.googleapis.com/knative-releases/${bucket}/latest/${file}" \
        --create-dirs \
        -o "${REPO_ROOT_DIR}/${target_dir}/${file}"
    fi
  done

done <<< "$(yq read --tojson "${REPO_ROOT_DIR}/hack/releases.yaml" '[*]')"