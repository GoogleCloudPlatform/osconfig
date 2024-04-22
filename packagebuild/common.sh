#!/bin/bash
# Copyright 2019 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

function build_success() {
  echo "Build succeeded: $@"
  exit 0
}

function build_fail() {
  echo "Build failed: $@"
  exit 1
}

function exit_error() {
  build_fail "$0:$1 \"$BASH_COMMAND\" returned $?"
}

trap 'exit_error $LINENO' ERR

function install_go() {
  # Installs a specific version of go for compilation, since availability varies
  # across linux distributions. Needs curl and tar to be installed.
  local arch="amd64"
  if [[ `uname -m` == "aarch64" ]]; then
    arch="arm64"
  fi

  local GOLANG="go1.22.2.linux-${arch}.tar.gz"
  export GOPATH=/usr/share/gocode
  export GOCACHE=/tmp/.cache

  # Golang setup
  [[ -d /tmp/go ]] && rm -rf /tmp/go
  mkdir -p /tmp/go/
  curl -s "https://dl.google.com/go/${GOLANG}" -o /tmp/go/go.tar.gz
  tar -C /tmp/go/ --strip-components=1 -xf /tmp/go/go.tar.gz

  export PATH="/tmp/go/bin:${GOPATH}/bin:${PATH}"  # set path for whoever invokes this function.
  export GO=/tmp/go/bin/go  # reference this go explicitly.
}

function git_checkout() {
  # Checks out a repo at a specified commit or ref into a specified directory.

  BASE_REPO="$1"
  REPO="$2"
  PULL_REF="$3"

  # pull the repository from github - start
  mkdir -p $REPO
  cd $REPO
  git init

  # fetch only the branch that we want to build
  git_command="git fetch https://github.com/${BASE_REPO}/${REPO}.git ${PULL_REF:-"master"}"
  echo "Running ${git_command}"
  $git_command

  git checkout FETCH_HEAD
}

function try_command() {
  n=0
  while ! "$@"; do
    echo "try $n to run $@"
    if [[ n -gt 3 ]]; then
      return 1
    fi
    ((n++))
    sleep 5
  done
}

function deploy_sbomutil() {
  if [ -z "${SBOM_UTIL_GCS_ROOT}" ]; then
    echo "SBOM_UTIL_GCS_ROOT is not defined, skipping sbomutil deployment..."
    return
  fi

  SBOM_UTIL_GCS_ROOT="${SBOM_UTIL_GCS_ROOT}/linux"

  # suffix the gcs path with arm64 if that's the case
  if [ "$(uname -m)" == "aarch64" ]; then
    SBOM_UTIL_GCS_ROOT="${SBOM_UTIL_GCS_ROOT}_arm64"
  fi

  export SBOM_UTIL=$(realpath "${PWD}/sbomutil")
  export SBOM_DIR="${PWD}"

  # Determine the latest sbomutil gcs path if available
  if [ -n "${SBOM_UTIL_GCS_ROOT}" ]; then
    SBOM_UTIL_GCS_PATH=$(gsutil ls $SBOM_UTIL_GCS_ROOT | tail -1)
  fi

  # Fetch sbomutil from gcs if available
  if [ -n "${SBOM_UTIL_GCS_PATH}" ]; then
    echo "Fetching sbomutil: ${SBOM_UTIL_GCS_PATH}"
    gsutil cp "${SBOM_UTIL_GCS_PATH%/}/sbomutil" "${SBOM_UTIL}"

    if [ -e "${SBOM_UTIL}" ]; then
      chmod +x "${SBOM_UTIL}"
    else
      echo "Failed to fetch sbomutil, file not copied locally"
    fi
  fi
}

function generate_and_push_sbom() {
  if [ -z "${SBOM_UTIL}" ]; then
    echo "deploy_sbomutil() was never attempted, skipping..."
    return
  fi

  if [ ! -e "${SBOM_UTIL}" ]; then
    echo "The sbomutil tool was not found, skipping sbom generation"
    return
  fi

  local BUILD_DIR=$1
  local BINARY_FILE=$2
  local PKGNAME=$3
  local VERSION=$4

  SBOM_FILE=$(realpath "${SBOM_DIR}/${PKGNAME}-${VERSION}.sbom.json")
  if [ -e "${SBOM_FILE}" ]; then
    echo "SBOM was already generated for this package, skipping..."
    return
  fi

  ${SBOM_UTIL} -archetype=source -comp_name="${PKGNAME}" -pkg_source="${BUILD_DIR}" \
    -pkg_binary="${BINARY_FILE}" -output="${SBOM_FILE}"

  echo "copying ${SBOM_FILE} to $GCS_PATH/"
  gsutil cp -n ${SBOM_FILE} "$GCS_PATH/"
}
