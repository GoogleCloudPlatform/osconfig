#!/bin/bash
#
# Produce a single package build with a custom version.
#
# - Package TYPEs can be found in the 'workflows/' directory as
#   a filename pattern  build_{{type}}.wf.json
# - Package Version should start with a digit
#
# Example (requires daisy tool somewhere in the PATH):
#  build rpm package for el9 like distributions and version 0.0.1 for
#  the last commit in the 'trustca-sync' branch of vorakl/guest-oslogin
#  fork on Github and save results to gs://vorakl-dev-builds/packages/ bucket.
#
#    env TYPE=el9 \
#        PROJECT=vorakl-dev \
#        ZONE=us-west1-a \
#        OWNER=vorakl \
#        REPO=guest-oslogin \
#        GIT_REF=trustca-sync \
#        GCS_PATH=gs://vorakl-dev-builds/packages \
#        VERSION=0.01 \
#        BUILD_DIR=. \
#    ./test_build.sh 

DEFAULT_TYPE='deb11'
DEFAULT_PROJECT='gcp-guest'
DEFAULT_ZONE='us-west1-a'
DEFAULT_OWNER='GoogleCloudPlatform'
DEFAULT_GIT_REF='master'
DEFAULT_GCS_PATH='${SCRATCHPATH}/packages'
DEFAULT_BUILD_DIR='.'
DEFAULT_VERSION='1dummy'

[[ -z "${TYPE}" ]] && read -p "Build type [${DEFAULT_TYPE}]: " TYPE
[[ -z "${PROJECT}" ]] && read -p "Build project [${DEFAULT_PROJECT}]: " PROJECT
[[ -z "${ZONE}" ]] && read -p "Build zone [${DEFAULT_ZONE}]: " ZONE
[[ -z "${OWNER}" ]] && read -p "Repo owner or org [${DEFAULT_OWNER}]: " OWNER
[[ -z "${GIT_REF}" ]] && read -p "Ref [${DEFAULT_GIT_REF}]: " GIT_REF
[[ -z "${GCS_PATH}" ]] && read -p "GCS Path to upload to [${DEFAULT_GCS_PATH}]: " GCS_PATH
[[ -z "${BUILD_DIR}" ]] && read -p "Directory to build from [${DEFAULT_BUILD_DIR}]: " BUILD_DIR
[[ -z "${REPO}" ]] && read -p "Repo name: " REPO

[[ -z "${TYPE}" ]] && TYPE=${DEFAULT_TYPE}
[[ -z "${PROJECT}" ]] && PROJECT=${DEFAULT_PROJECT}
[[ -z "${ZONE}" ]] && ZONE=${DEFAULT_ZONE}
[[ -z "${OWNER}" ]] && OWNER=${DEFAULT_OWNER}
[[ -z "${GIT_REF}" ]] && GIT_REF=${DEFAULT_GIT_REF}
[[ -z "${GCS_PATH}" ]] && GCS_PATH=${DEFAULT_GCS_PATH}
[[ -z "${BUILD_DIR}" ]] && BUILD_DIR=${DEFAULT_BUILD_DIR}
[[ -z "${VERSION}" ]] && VERSION=${DEFAULT_VERSION}

WF="workflows/build_${TYPE}.wf.json"

if [[ ! -f "${WF}" ]]; then
  echo "Unknown build type ${TYPE}"
  exit 1
fi

set -x
daisy \
  -project ${PROJECT} \
  -zone ${ZONE} \
  -var:gcs_path=${GCS_PATH} \
  -var:repo_owner=${OWNER} \
  -var:repo_name=${REPO} \
  -var:git_ref=${GIT_REF} \
  -var:build_dir=${BUILD_DIR} \
  -var:version=${VERSION} \
  "${WF}"
