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

apt-get install -y git-core
<<<<<<< HEAD:packaging/build_packages_goo.sh
git clone "https://github.com/${BASE_REPO}/osconfig.git" 
cd osconfig
packaging/setup_goo.sh
gsutil cp google-osconfig-agent*.goo "${GCS_PATH}/"
=======
git clone "https://github.com/GoogleCloudPlatform/osconfig.git"
cd osconfig

agent-packaging/packaging-scripts/setup_goo.sh
gsutil cp google-osconfig-agent*.goo "gs://osconfig-agent-package/"
>>>>>>> Add osconfig agent packaging scripts and docker file:agent-packaging/packaging-scripts/build_packages_goo.sh

echo 'Package build success'
