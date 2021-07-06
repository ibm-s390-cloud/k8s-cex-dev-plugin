# Copyright 2021 IBM Corp.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Dockerfile with go build for the s390 cex plugin v1.0
# Author(s): Harald Freudenberger <freude@de.ibm.com>
#

FROM golang:1.14 as build

# make build dir /build
WORKDIR /build

# Copy and download dependency using go mod
COPY go.mod go.sum ./
RUN go mod download

# Copy the code into the build dir
COPY ap.go cryptoconfigs.go main.go plugin.go podlister.go shadowsysfs.go zcrypt.go ./

# Build the application
RUN CGO_ENABLED=0 GO111MODULE=on go build -o cex-plugin .

# now do the runtime image
FROM scratch

WORKDIR /work
COPY --from=build /build/cex-plugin ./

LABEL name="cex-plugin-v1.0" \
  description="A Kubernetes device plugin for s390 supporting CEX crypto cards"

ENTRYPOINT ["/work/cex-plugin"]
