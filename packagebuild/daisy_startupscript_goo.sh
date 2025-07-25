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

URL="http://metadata/computeMetadata/v1/instance/attributes"
GCS_PATH=$(curl -f -H Metadata-Flavor:Google ${URL}/daisy-outs-path)
SRC_PATH=$(curl -f -H Metadata-Flavor:Google ${URL}/daisy-sources-path)
REPO_OWNER=$(curl -f -H Metadata-Flavor:Google ${URL}/repo-owner)
REPO_NAME=$(curl -f -H Metadata-Flavor:Google ${URL}/repo-name)
GIT_REF=$(curl -f -H Metadata-Flavor:Google ${URL}/git-ref)
BUILD_DIR=$(curl -f -H Metadata-Flavor:Google ${URL}/build-dir)
VERSION=$(curl -f -H Metadata-Flavor:Google ${URL}/version)

echo "Started build..."

gsutil cp "${SRC_PATH}/common.sh" ./
. common.sh

# disable the backports repo for debian-10
sed -i 's/^.*debian buster-backports main.*$//g' /etc/apt/sources.list
# disable the backports repo for debian-11
sed -i 's/^.*debian bullseye-backports main.*$//g' /etc/apt/sources.list

try_command apt-get -y update
try_command apt-get install -y --no-install-{suggests,recommends} git-core

# We always install go, needed for goopack.
echo "Installing go"
install_go

# Install goopack.
GO111MODULE=on $GO install -v github.com/google/googet/v2/goopack@latest

git_checkout "$REPO_OWNER" "$REPO_NAME" "$GIT_REF"
if [[ -n "$BUILD_DIR" ]]; then
    cd "$BUILD_DIR"
fi

if find . -type f -iname '*.go' >/dev/null; then
  echo "Installing go dependencies"
  $GO mod download
fi

echo "Building package(s)"
for spec in packaging/googet/*.goospec; do
  goopack -var:version="$VERSION" "$spec"
  name=$(basename "${spec}")
  pref=${name%.*}
done

gsutil cp -n *.goo "$GCS_PATH/"
build_success "Built `ls *.goo|xargs`"
