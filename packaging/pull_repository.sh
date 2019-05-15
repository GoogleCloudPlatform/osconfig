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

# pull the repository from github - start
mkdir -p $REPO
cd $REPO
git init

# fetch only the branch that we want to build
GIT_CMD="git fetch https://github.com/${BASE_REPO}/${REPO}.git"

if [[ "$PULL_NUMBER" != "" ]]; then
  GIT_CMD+=" pull/${PULL_NUMBER}/head:pr-${PULL_NUMBER}"
fi

echo "Running $GIT_CMD"
$GIT_CMD

# pull the repository from github - end