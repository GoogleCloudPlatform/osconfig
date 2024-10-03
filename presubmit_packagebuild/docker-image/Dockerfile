# Copyright 2018 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
FROM gcr.io/compute-image-tools/daisy:latest

#FROM alpine
# Needed for gsutil.
FROM google/cloud-sdk:alpine

RUN apk add --no-cache bash

WORKDIR /
COPY --from=0 /daisy daisy
# Daisy image copies this from gcr.io/distroless/base, we copy here.
COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

# Must be built from root of repo, where packagebuild dir is.
COPY packagebuild packagebuild
# Copy this Dockerfile for debugging.
COPY presubmit_packagebuild/docker-image/Dockerfile Dockerfile
COPY presubmit_packagebuild/docker-image/main.sh main.sh

ENTRYPOINT ["/main.sh"]
