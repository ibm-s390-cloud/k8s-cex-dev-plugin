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
NAME := ibm-cex-device-plugin-cm
VERSION := $(shell $(PWD)/version.sh -v nightly)
IMAGE := $(REGISTRY)$(NAME):$(VERSION)
IMAGE_LATEST := $(REGISTRY)$(NAME):latest
RUNTIME := podman
RELEASE := $(shell $(PWD)/version.sh -r)
GIT_URL := $(shell $(PWD)/version.sh --git-url)
GIT_COMMIT := $(shell $(PWD)/version.sh --git-commit)

.PHONY: build
build:
	$(RUNTIME) build -f Dockerfile -t $(IMAGE) --build-arg VERSION=$(VERSION) \
	    --build-arg RELEASE=$(RELEASE) --build-arg GIT_URL=$(GIT_URL) \
	    --build-arg GIT_COMMIT=$(GIT_COMMIT) .

.PHONY: push
push:
	if test -n "$(REGISTRY)"; then \
		$(RUNTIME) push $(IMAGE); \
	fi

.PHONY: tag-latest
tag-latest:
	$(RUNTIME) tag $(IMAGE) $(IMAGE_LATEST)

.PHONY: push-latest
push-latest:
	if test -n "$(REGISTRY)"; then \
		$(RUNTIME) push $(IMAGE_LATEST); \
	fi

.PHONY: show-tag
show-tag:
	@echo $(TAG)
