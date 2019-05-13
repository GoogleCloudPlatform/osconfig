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

# centos6 has some issues with network on first boot
el6_install(){
  n=0
  while ! yum install -y https://centos6.iuscommunity.org/ius-release.rpm; do
    if [[ n -gt 3 ]]; then
      exit 1
    fi
    n=$[$n+1]
    sleep 5
  done

  rpm --import /etc/pki/rpm-gpg/RPM-GPG-KEY-IUS-6
}

el7_install(){
  yum install -y https://centos7.iuscommunity.org/ius-release.rpm
  rpm --import /etc/pki/rpm-gpg/RPM-GPG-KEY-IUS-7
}

# Install git2 as this is not avaiable in centos 6/7
RELEASE_RPM=$(rpm -qf /etc/redhat-release)
RELEASE=$(rpm -q --qf '%{VERSION}' ${RELEASE_RPM})
case ${RELEASE} in
  6*) el6_install;;
  7*) el7_install;;
esac
yum install -y git2u

packaging/pull_repository.sh

packaging/setup_rpm.sh
gsutil cp /tmp/rpmpackage/RPMS/x86_64/google-osconfig-agent-*.rpm "${GCS_PATH}/"

echo 'Package build success'
