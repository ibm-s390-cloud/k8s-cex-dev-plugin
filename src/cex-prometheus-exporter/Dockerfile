# Copyright 2022 IBM Corp.
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

ARG BUILD_IMAGE=quay.io/projectquay/golang:1.23

FROM ${BUILD_IMAGE} as build

# define build arguments, as buildah needs them after the FROM
ARG VERSION
ARG GIT_URL
ARG GIT_COMMIT

# some images set the user explicitly, revert to root for building
USER root

# make build dir /build
WORKDIR /build

# Copy and download dependency using go mod
COPY go.mod go.sum ./
RUN go mod download

# Copy the code into the build dir
COPY collector.go disposer.go main.go promstuff.go ./

# Build the application
RUN CGO_ENABLED=0 GO111MODULE=on go build -v -o cex-prometheus-exporter \
    -ldflags="-X 'main.version=${VERSION}' -X 'main.git_url=${GIT_URL}' \
    -X 'main.git_commit=${GIT_COMMIT}'" .

# now do the runtime image
FROM scratch

# define build arguments, as buildah needs them after the FROM
ARG VERSION
ARG RELEASE
ARG GIT_URL
ARG GIT_COMMIT
ARG LABELNAME

WORKDIR /licenses
COPY LICENSE license

WORKDIR /work
COPY --from=build /build/cex-prometheus-exporter ./

LABEL name="${LABELNAME}" \
      description="Prometheus exporter for the CEX device plug-in for IBM zSystems and LinuxONE (s390x)" \
      summary="Prometheus exporter for the CEX device plug-in for IBM zSystems and LinuxONE (s390x)" \
      maintainer="IBM" \
      vendor="IBM" \
      version="${VERSION}" \
      release="${RELEASE}" \
      git_url="${GIT_URL}" \
      git_commit="${GIT_COMMIT}"

ENTRYPOINT ["/work/cex-prometheus-exporter"]
