#!/usr/bin/make -f
#
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

.PHONY: build
build:	build-cex-device-plugin-image

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
