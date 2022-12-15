#!/usr/bin/make -f
#
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
# Makefile to build the IBM Cex Device Plugin
# Author(s): Hendrik Brueckner <brueckner@linux.ibm.com>
#

# Registry (ending with /), leave blank for using localhost
REGISTRY :=
NAME := ibm-cex-plugin-cm
VERSION := $(shell $(PWD)/version.sh -v)
LABELNAME := $(NAME)-$(VERSION)
IMAGE := $(REGISTRY)$(NAME):$(VERSION)
IMAGE_LATEST := $(REGISTRY)$(NAME):latest
RUNTIME := podman
RELEASE := $(shell $(PWD)/version.sh -r)
GIT_URL := $(shell $(PWD)/version.sh --git-url)
GIT_COMMIT := $(shell $(PWD)/version.sh --git-commit)

# The major build is building an image with the cex device plugin
# application and the cex prometheus application baked in. So the
# behavior of the image depends on what entry point is actually
# invoked. This image build is the default target when no arguments
# are given to make.
# However, sometimes you want to have individual images for each
# of the applications. So there are make targets provided here
# to build an image with only the cex device plugin inside or
# an image with only the cex prometheus exporter inside.

.PHONY: build
build: build-cex-plugin-and-exporter-image

.PHONY: build-cex-plugin-and-exporter-image
build-cex-plugin-and-exporter-image:
	cd src && \
	$(RUNTIME) build -f Dockerfile -t $(IMAGE) --build-arg VERSION=$(VERSION) \
	    --build-arg RELEASE=$(RELEASE) --build-arg GIT_URL=$(GIT_URL) \
	    --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg LABELNAME=$(LABELNAME) .

.PHONY: buildah-cex-plugin-and-exporter-image
buildah-cex-plugin-and-exporter-image:
	cd src && \
	buildah bud -f Dockerfile -t $(IMAGE) --build-arg VERSION=$(VERSION) \
	    --build-arg RELEASE=$(RELEASE) --build-arg GIT_URL=$(GIT_URL) \
	    --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg LABELNAME=$(LABELNAME) .


.PHONY: build-cex-device-plugin-image
build-cex-device-plugin-image:
	cd src/cex-device-plugin && \
	$(RUNTIME) build -f Dockerfile -t $(IMAGE) --build-arg VERSION=$(VERSION) \
	    --build-arg RELEASE=$(RELEASE) --build-arg GIT_URL=$(GIT_URL) \
	    --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg LABELNAME=$(LABELNAME) .

.PHONY: buildah-cex-device-plugin-image
buildah-cex-device-plugin-image:
	cd src/cex-device-plugin && \
	buildah bud -f Dockerfile -t $(IMAGE) --build-arg VERSION=$(VERSION) \
	    --build-arg RELEASE=$(RELEASE) --build-arg GIT_URL=$(GIT_URL) \
	    --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg LABELNAME=$(LABELNAME) .

.PHONY: build-cex-prometheus-exporter-image
build-cex-prometheus-exporter-image:
	cd src/cex-prometheus-exporter && \
	$(RUNTIME) build -f Dockerfile -t $(IMAGE) --build-arg VERSION=$(VERSION) \
	    --build-arg RELEASE=$(RELEASE) --build-arg GIT_URL=$(GIT_URL) \
	    --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg LABELNAME=$(LABELNAME) .

.PHONY: buildah-cex-prometheus-exporter-image
buildah-cex-prometheus-exporter-image:
	cd src/cex-prometheus-exporter && \
	buildah bud -f Dockerfile -t $(IMAGE) --build-arg VERSION=$(VERSION) \
	    --build-arg RELEASE=$(RELEASE) --build-arg GIT_URL=$(GIT_URL) \
	    --build-arg GIT_COMMIT=$(GIT_COMMIT) --build-arg LABELNAME=$(LABELNAME) .

.PHONY: push-image
push-image:
	if test -n "$(REGISTRY)"; then \
		$(RUNTIME) push $(IMAGE); \
	fi

.PHONY: tag-latest-image
tag-latest-image:
	$(RUNTIME) tag $(IMAGE) $(IMAGE_LATEST)

.PHONY: push-latest-image
push-latest-image:
	if test -n "$(REGISTRY)"; then \
		$(RUNTIME) push $(IMAGE_LATEST); \
	fi
