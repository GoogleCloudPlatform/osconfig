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


echo "started build..."

apt-get -y update && apt-get -y upgrade
apt-get install -y git-core
git clone --branch ${PULL_REF} "https://github.com/${BASE_REPO}/${REPO}.git"

cd osconfig

<<<<<<< HEAD:agent-packaging/packaging-scripts/build_packages_deb.sh
<<<<<<< HEAD:packaging/build_packages_deb.sh
apt-get install -y git-core 
git clone "https://github.com/${BASE_REPO}/osconfig.git"
cd osconfig
packaging/setup_deb.sh 
<<<<<<< HEAD:packaging/build_packages_deb.sh
gsutil cp /tmp/debpackage/google-osconfig-agent*.deb "${GCS_PATH}/"
=======
gsutil cp /tmp/debpackage/google-osconfig-agent*.deb "${GCS_PATH}/" 
=======
source ./agent-packaging/packaging-scripts/setup_deb.sh
=======
source ./packaging/setup_deb.sh
<<<<<<< HEAD
<<<<<<< HEAD
>>>>>>> Move back  packaging code to where it was:packaging/build_packages_deb.sh
gsutil cp /tmp/debpackage/google-osconfig-agent*.deb "gs://osconfig-agent-package/"
>>>>>>> Add osconfig agent packaging scripts and docker file:agent-packaging/packaging-scripts/build_packages_deb.sh
<<<<<<< HEAD
>>>>>>> Add osconfig agent packaging scripts and docker file:agent-packaging/packaging-scripts/build_packages_deb.sh
=======
=======
gsutil cp /tmp/debpackage/google-osconfig-agent*.deb "${PKG_GCS_OUT_DIR}/"
>>>>>>> Use environment variables replacements instead of hard coding
<<<<<<< HEAD
>>>>>>> Use environment variables replacements instead of hard coding
=======
=======
gsutil cp /tmp/debpackage/google-osconfig-agent*.deb "${GCS_PATH}/"
>>>>>>> Change env variable used to dump artifacts
>>>>>>> Change env variable used to dump artifacts

echo 'Package build success'
