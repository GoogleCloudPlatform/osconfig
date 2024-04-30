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

URL="http://169.254.169.254/computeMetadata/v1/instance/attributes"
GCS_PATH=$(curl -f -H Metadata-Flavor:Google ${URL}/daisy-outs-path)
SRC_PATH=$(curl -f -H Metadata-Flavor:Google ${URL}/daisy-sources-path)
REPO_OWNER=$(curl -f -H Metadata-Flavor:Google ${URL}/repo-owner)
REPO_NAME=$(curl -f -H Metadata-Flavor:Google ${URL}/repo-name)
GIT_REF=$(curl -f -H Metadata-Flavor:Google ${URL}/git-ref)
BUILD_DIR=$(curl -f -H Metadata-Flavor:Google ${URL}/build-dir)
VERSION=$(curl -f -H Metadata-Flavor:Google ${URL}/version)
VERSION=${VERSION:="1dummy"}

DEBIAN_FRONTEND=noninteractive

echo "Started build..."

gsutil cp "${SRC_PATH}/common.sh" ./
. common.sh

# disable the backports repo for debian-10
sed -i 's/^.*debian buster-backports main.*$//g' /etc/apt/sources.list

try_command apt-get -y update
try_command apt-get install -y --no-install-{suggests,recommends} git-core \
  debhelper devscripts build-essential equivs libdistro-info-perl

git_checkout "$REPO_OWNER" "$REPO_NAME" "$GIT_REF"

if [[ -n "$BUILD_DIR" ]]; then
    cd "$BUILD_DIR"
fi

PKGNAME="$(grep "^Package:" ./packaging/debian/control|cut -d' ' -f2-)"

# Install build deps
mk-build-deps -t "apt-get -o Debug::pkgProblemResolver=yes \
  --no-install-recommends --yes" --install packaging/debian/control
dpkg-checkbuilddeps packaging/debian/control

if grep -q '+deb' packaging/debian/changelog; then
  DEB=$(</etc/debian_version)
  # Currently Debian 11 doesn't use a numerical version number in its release.
  if [[ "${DEB}" =~ "bullseye" ]]; then
    DEB="11"
  elif [[ "${DEB}" =~ "bookworm" ]]; then
    DEB="12"
  fi
  DEB="+deb${DEB/.*}"
fi

if grep -q 'golang' packaging/debian/control; then
  echo "Installing go"
  install_go

  echo "Installing go dependencies"
  $GO mod download
fi

COMMITURL="https://github.com/$REPO_OWNER/$REPO_NAME/tree/$(git rev-parse HEAD)"

echo "Building package(s)"

# Create build dir.
BUILD_DIR="/tmp/debpackage"
[[ -d $BUILD_DIR ]] || mkdir $BUILD_DIR

# Create 'upstream' tarball.
TAR="${PKGNAME}_${VERSION}.orig.tar.gz"
tar czvf "${BUILD_DIR}/${TAR}" --exclude .git --exclude packaging \
  --transform "s/^\./${PKGNAME}-${VERSION}/" .

# Extract tarball and build.
tar -C "$BUILD_DIR" -xzvf "${BUILD_DIR}/${TAR}"
PKGDIR="${BUILD_DIR}/${PKGNAME}-${VERSION}"
cp -r packaging/debian "${BUILD_DIR}/${PKGNAME}-${VERSION}/"

cd "${BUILD_DIR}/${PKGNAME}-${VERSION}"

sed -i"" "/^Source:/aXB-Git: ${COMMITURL}" debian/control

# We generate this to enable auto-versioning.
[[ -f debian/changelog ]] && rm debian/changelog
RELEASE="g1${DEB}"
dch --create -M -v 1:${VERSION}-${RELEASE} --package $PKGNAME -D stable \
  "Debian packaging for ${PKGNAME}"
DEB_BUILD_OPTIONS="noautodbgsym nocheck" debuild -e "VERSION=${VERSION}" -e "RELEASE=${RELEASE}" -us -uc

for deb in $BUILD_DIR/*.deb; do
  dpkg-deb -I $deb
  dpkg-deb -c $deb
done

echo "copying $BUILD_DIR/*.deb to $GCS_PATH/"
gsutil cp -n $BUILD_DIR/*.deb "$GCS_PATH/"
build_success Built $BUILD_DIR/*.deb
