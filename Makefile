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
TAG := $(shell $(PWD)/taggen.sh)
IMAGE := $(REGISTRY)$(NAME):$(TAG)
IMAGE_LATEST := $(REGISTRY)$(NAME):latest
RUNTIME := podman

.PHONY: build
build:
	$(RUNTIME) build -f Dockerfile -t $(IMAGE)

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
