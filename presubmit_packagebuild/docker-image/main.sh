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

PROJECT="$1"
ZONE="$2"
DISTROS="$3"            # Distros to build
GCS_OUTPUT_BUCKET="$4"  # Destination for artifacts
BUILD_DIR="$5"          # Directory to build from
VERSION=""

# Workflow consisting entirely of separate IncludeWorkflow steps referencing
# build_${distro}.wf.json, which should be checked out from guest-test-infra.
function generate_build_workflow() {
  local WF="$1"

  config='{
  "Name": "build-packages",
  "DefaultTimeout": "30m",
  "Vars": {
    "gcs_path": {
      "Value": "${SCRATCHPATH}/packages",
      "Description": "GCS path for built packages e.g. gs://my-bucket/packages"
    },
    "repo_owner": {
      "Value": "GoogleCloudPlatform",
      "Description": "GitHub repo owner or organization"
    },
    "repo_name": {
      "Description": "Github repo name",
      "Required": true
    },
    "git_ref": {
      "Value": "master",
      "Description": "Git ref to check out and build"
    },
    "version": {
      "Description": "Version to build"
    },
    "build_dir": {
      "Description": "Directory to build from"
    }
  },
  "Steps": {'

  for distro in ${DISTROS//,/ }; do
    distrodash=$(echo "$distro" | tr '_' '-')
    if [[ "$config" =~ IncludeWorkflow ]]; then
      # Append additional steps
      config="${config},"
    fi
    config="${config}\n"'    "'"$distrodash"'": {
      "IncludeWorkflow": {
        "Path": "./workflows/build_'"$distro"'.wf.json",
        "Vars": {
          "gcs_path": "${gcs_path}",
          "repo_owner": "${repo_owner}",
          "repo_name": "${repo_name}",
          "git_ref": "${git_ref}",
          "build_dir": "${build_dir}",
          "version": "${version}"
        }
      }
    }'
  done

  config="$config"'
  }
}'
  echo -e "$config" > "$WF"
}

# Sets service account used for daisy and gsutil commands below. Will use
# default service account for VM or k8s node if not set.
if [[ -n $GOOGLE_APPLICATION_CREDENTIALS ]]; then
  gcloud auth activate-service-account --key-file=$GOOGLE_APPLICATION_CREDENTIALS
fi

cd /packagebuild

WF="build.wf.json"
generate_build_workflow "$WF"
echo "Generated workflow:"
cat "$WF"

## Some vars such as REPO_OWNER and PULL_NUMBER are set by prow
DAISY_VARS="repo_owner=${REPO_OWNER},repo_name=${REPO_NAME}"

## only add pull reference in case of presubmit jobs
if [[ "$JOB_TYPE" == "presubmit" ]]; then
  DAISY_VARS+=",git_ref=pull/${PULL_NUMBER}/head"
else
  DAISY_VARS+=",git_ref=${PULL_BASE_REF}"
fi

## Build from subdir if requested
if [[ -n "$BUILD_DIR" ]]; then
  DAISY_VARS+=",build_dir=${BUILD_DIR}"
fi

DAISY_CMD="/daisy -project ${PROJECT} -zone ${ZONE} -variables ${DAISY_VARS} ${WF}"

echo "Running daisy workflow for package builds"
echo "Daisy command: ${DAISY_CMD}"
$DAISY_CMD 2>err | tee out
if [[ ${PIPESTATUS[0]} -ne 0 ]]; then
  echo "error running daisy: stderr: $(<err)"
  exit 1
fi

# TODO: pass this in
pattern="https://console.cloud.google.com/storage/browser/"
DAISY_BUCKET="gs://$(sed -En "s|(^.*)$pattern||p" out)"

# copy daisy logs and artifacts to artifacts folder for prow
# $ARTIFACTS is set by prow
if [[ -n $ARTIFACTS ]]; then
  echo "copying daisy outputs from ${DAISY_BUCKET}/packages to prow artifacts dir"
  gsutil cp "${DAISY_BUCKET}/packages/*" ${ARTIFACTS}/
fi
