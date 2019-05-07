#!/bin/bash
# Copyright 2018 Google Inc. All Rights Reserved.
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

echo "running common script..."
source ./agent-packaging/packaging-scripts/common.sh

# DEB creation tools.
DEBIAN_FRONTEND=noninteractive  apt-get -y install debhelper devscripts build-essential curl tar

# Build dependencies.
<<<<<<< HEAD:packaging/setup_deb.sh
DEBIAN_FRONTEND=noninteractive sudo apt-get -y install dh-golang dh-systemd golang-go  # golang-go is unused but required for debuild being happy with static binaries.
=======
DEBIAN_FRONTEND=noninteractive  apt-get -y install dh-golang dh-systemd golang-go
>>>>>>> Add osconfig agent packaging scripts and docker file:agent-packaging/packaging-scripts/setup_deb.sh

dpkg-checkbuilddeps ./agent-packaging/packaging-scripts/debian/control

[[ -d /tmp/debpackage ]] && rm -rf /tmp/debpackage
mkdir /tmp/debpackage
tar czvf /tmp/debpackage/${NAME}_${VERSION}.orig.tar.gz --exclude .git --exclude agent-packaging --transform "s/^\./${NAME}-${VERSION}/" .

pushd /tmp/debpackage
tar xzvf ${NAME}_${VERSION}.orig.tar.gz

cd ${NAME}-${VERSION}

cp -r ${working_dir}/agent-packaging/packaging-scripts/debian ./
cp -r ${working_dir}/agent-packaging/packaging-scripts/*.service ./debian/

debuild -e "VERSION=${VERSION}" -us -uc

popd
