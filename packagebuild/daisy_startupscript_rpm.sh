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

# This script is expected to be run on an Enterprise Linux system (RHEL, CentOS)
# on GCE with various flags set in the instance metadata including the git
# repository to clone. The script produces an RPM defined by an RPM spec in the
# packaging/ directory from the cloned repo.

URL="http://metadata/computeMetadata/v1/instance/attributes"
function get_md() {
  curl -f -H Metadata-Flavor:Google "${URL}/${1}"
}
GCS_PATH=$(get_md daisy-outs-path)
SRC_PATH=$(get_md daisy-sources-path)
REPO_OWNER=$(get_md repo-owner)
REPO_NAME=$(get_md repo-name)
GIT_REF=$(get_md git-ref)
BUILD_DIR=$(get_md build-dir)
VERSION=$(get_md version)
VERSION=${VERSION:-"dummy"}
SBOM_UTIL_GCS_ROOT=$(get_md sbom-util-gcs-root)

echo "Started build..."

# common.sh contains functions common to all builds.
gsutil cp "${SRC_PATH}/common.sh" ./
. common.sh

deploy_sbomutil

# Install git2 as this is not available in centos 6/7
VERSION_ID=6
if [[ -f /etc/os-release ]]; then
  eval $(grep VERSION_ID /etc/os-release)
  VERSION_ID=${VERSION_ID:0:1}
fi

GIT="git"
if [[ ${VERSION_ID} =~ 6|7 ]]; then
  try_command yum install -y "https://repo.ius.io/ius-release-el${VERSION_ID}.rpm"
  rpm --import /etc/pki/rpm-gpg/RPM-GPG-KEY-IUS-${VERSION_ID}
  GIT="git236"
fi

# Install DevToolSet with gcc 10 for EL7.
# Centos 7 has only gcc 4.8.5 available.
if (( ${VERSION_ID} == 7 )); then
  try_command yum install -y centos-release-scl
  try_command yum install -y devtoolset-10-gcc-c++.x86_64
fi

# Enable CRB repo on EL9.
if [[ ${VERSION_ID} = 9 ]]; then
  eval $(grep ID /etc/os-release)
  # RHEL has a different CRB repo than Rocky/CentOS.
  if [[ ${ID} == "rhel" ]]; then
    dnf config-manager --set-enabled rhui-codeready-builder-for-rhel-9-$(uname -m)-rhui-rpms
  else
    dnf config-manager --set-enabled crb
  fi
fi

try_command yum install -y $GIT rpmdevtools yum-utils python3-devel

git_checkout "$REPO_OWNER" "$REPO_NAME" "$GIT_REF"

if [[ -n "$BUILD_DIR" ]]; then
  cd "$BUILD_DIR"
fi

if grep -q '%{_go}' ./packaging/*.spec; then
  echo "Installing go"
  install_go

  echo "Installing go dependencies"
  $GO mod download
fi

# Make build dirs as needed.
RPMDIR=/usr/src/redhat
for dir in ${RPMDIR}/{SOURCES,SPECS}; do
  [[ -d "$dir" ]] || mkdir -p "$dir"
done

# Find the RPM specs to build for this version.
TOBUILD=""
SPECS=$(ls ./packaging/*.spec | sed -re 's/(\.el.)?.spec//' | sort -ru)
echo "all specs $SPECS"
for spec in $SPECS; do
  spec=$(basename "$spec")
  distspec="${spec}.el${VERSION_ID}.spec"
  echo checking $spec and $distspec
  if [[ -f "./packaging/$distspec" ]]; then
    TOBUILD="${TOBUILD} ${distspec}"
  else
    TOBUILD="${TOBUILD} ${spec}.spec"
  fi
done

[[ -z "$TOBUILD" ]] && build_fail "No RPM specs found"

COMMITURL="https://github.com/$REPO_OWNER/$REPO_NAME/tree/$(git rev-parse HEAD)"

echo "Building package(s)"

# Enable gcc 10 for EL7 only and before rpmbuild
if (( ${VERSION_ID} == 7 )); then
  source /opt/rh/devtoolset-10/enable
fi

for spec in $TOBUILD; do
  PKGNAME="$(grep Name: "./packaging/${spec}"|cut -d' ' -f2-|tr -d ' ')"
  yum-builddep -y "./packaging/${spec}"

  cp "./packaging/${spec}" "${RPMDIR}/SPECS/"
  cp ./packaging/*.tar.gz "${RPMDIR}/SOURCES/" || :
  cp ./packaging/*.patch "${RPMDIR}/SOURCES/" || :

  sed -i"" "/^Version/aVcs: ${COMMITURL}" "${RPMDIR}/SPECS/${spec}"

  tar czvf "${RPMDIR}/SOURCES/${PKGNAME}_${VERSION}.orig.tar.gz" \
    --exclude .git --exclude packaging \
    --transform "s/^\./${PKGNAME}-${VERSION}/" .

  rpmbuild --define "_topdir ${RPMDIR}/" --define "_version ${VERSION}" \
    --define "_go ${GO:-"UNSET"}" --define "_gopath ${GOPATH:-"UNSET"}" \
    -ba "${RPMDIR}/SPECS/${spec}"

  SRPM_FILE=$(find ${RPMDIR}/SRPMS -iname "${PKGNAME}*.src.rpm")
  generate_and_push_sbom ./ "${SRPM_FILE}" "${PKGNAME}" "${VERSION}"
done

rpms=$(find ${RPMDIR}/{S,}RPMS -iname "${PKGNAME}*.rpm")
for rpm in $rpms; do
  rpm -qpilR $rpm
done
echo "copying ${rpms} to $GCS_PATH/"
gsutil cp -n ${rpms} "$GCS_PATH/"
build_success "Built $(echo ${rpms}|xargs)"
