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
FROM us-docker.pkg.dev/artifact-foundry-prod/docker-3p-trusted/golang:1.25

RUN apt-get update && \
    apt-get install -y wget ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY . /source
WORKDIR /source

RUN go mod download && CGO_ENABLED=0 go build -o /e2e_tests main.go
RUN chmod +x /e2e_tests

FROM gcr.io/compute-image-tools-test/wrapper:latest

COPY --from=0 /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=0 /e2e_tests e2e_tests
ENTRYPOINT ["./wrapper", "./e2e_tests"]
