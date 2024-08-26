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
# Dockerfile with go build for the s390 cex device plugin and
# the cex prometheus exporter into one image
# Author(s): Harald Freudenberger <freude@de.ibm.com>
#

# define image to build cex-device-plugin & cex-prometheus-exporter
ARG BUILD_IMAGE=registry.hub.docker.com/library/golang:1.21

# cex-device-plugin build to build1
FROM ${BUILD_IMAGE} as build1

# define build arguments, as buildah needs them after the FROM
ARG VERSION
ARG GIT_URL
ARG GIT_COMMIT

# make build dir /build1
WORKDIR /build1

# Copy and download dependency using go mod
COPY cex-device-plugin/go.mod cex-device-plugin/go.sum ./
RUN go mod download

# Copy the code into the build dir
COPY cex-device-plugin/ap.go cex-device-plugin/cryptoconfigs.go \
     cex-device-plugin/main.go cex-device-plugin/plugin.go \
     cex-device-plugin/podlister.go cex-device-plugin/shadowsysfs.go \
     cex-device-plugin/zcrypt.go cex-device-plugin/metricscollector.go ./

# Build the application
RUN CGO_ENABLED=0 GO111MODULE=on go build -v -o cex-plugin \
    -ldflags="-X 'main.version=${VERSION}' -X 'main.git_url=${GIT_URL}' \
    -X 'main.git_commit=${GIT_COMMIT}'" .


# cex-prometheus-exporter build to build2

FROM ${BUILD_IMAGE} as build2

# define build arguments, as buildah needs them after the FROM
ARG VERSION
ARG GIT_URL
ARG GIT_COMMIT

# make build dir /build2
WORKDIR /build2

# Copy and download dependency using go mod
COPY cex-prometheus-exporter/go.mod cex-prometheus-exporter/go.sum ./
RUN go mod download

# Copy the code into the build dir
COPY cex-prometheus-exporter/collector.go cex-prometheus-exporter/disposer.go \
     cex-prometheus-exporter/main.go cex-prometheus-exporter/promstuff.go ./

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
COPY --from=build1 /build1/cex-plugin ./
COPY --from=build2 /build2/cex-prometheus-exporter ./

LABEL name="${LABELNAME}" \
      description="Kubernetes device plug-in for IBM CryptoExpress (CEX) cards for for IBM zSystems and LinuxONE (s390x)" \
      summary="Kubernetes device plug-in for IBM CryptoExpress (CEX) cards for for IBM zSystems and LinuxONE (s390x)" \
      maintainer="IBM" \
      vendor="IBM" \
      version="${VERSION}" \
      release="${RELEASE}" \
      git_url="${GIT_URL}" \
      git_commit="${GIT_COMMIT}"

# Here are two entry points showing the two possible ways how this image
# can be used: as cex-prometheus-exporter container or as container
# instances for the cex-device-plugin. If no command is given on invocation
# the last entry point wins. So by default the cex-device-plugin app is started.
ENTRYPOINT ["/work/cex-prometheus-exporter"]
ENTRYPOINT ["/work/cex-plugin"]
