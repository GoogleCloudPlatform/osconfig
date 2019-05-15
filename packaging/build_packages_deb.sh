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

set -e

function exit_error
{
  echo "build failed"
  exit 1
}

trap exit_error ERR

export URL="http://metadata/computeMetadata/v1/instance/attributes"
export GCS_PATH=$(curl -f -H Metadata-Flavor:Google ${URL}/daisy-outs-path)
export BASE_REPO=$(curl -f -H Metadata-Flavor:Google ${URL}/base-repo)
export REPO=$(curl -f -H Metadata-Flavor:Google ${URL}/repo)
export PULL_REF=$(curl -f -H Metadata-Flavor:Google ${URL}/pull-ref)

echo "started build..."

apt-get -y update && apt-get -y upgrade
apt-get install -y git-core

packaging/pull_repository.sh

packaging/setup_deb.sh

gsutil cp /tmp/debpackage/google-osconfig-agent*.deb "${GCS_PATH}/"

echo 'Package build success'
