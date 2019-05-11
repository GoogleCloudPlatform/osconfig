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

URL="http://metadata/computeMetadata/v1/instance/attributes"
GCS_PATH=$(curl -f -H Metadata-Flavor:Google ${URL}/daisy-outs-path)
BASE_REPO=$(curl -f -H Metadata-Flavor:Google ${URL}/base-repo)
REPO=$(curl -f -H Metadata-Flavor:Google ${URL}/repo)
PULL_REF=$(curl -f -H Metadata-Flavor:Google ${URL}/pull-ref)

apt-get install -y git-core

git clone --branch ${PULL_REF} "https://github.com/${BASE_REPO}/${REPO}.git"

cd osconfig
source ./packaging/setup_goo.sh
gsutil cp google-osconfig-agent*.goo "${GCS_PATH}/"

echo 'Package build success'
