#!/bin/sh
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
# Scrippt to tag the IBM Cex Device Plugin
# Author(s): Hendrik Brueckner <brueckner@linux.ibm.com>
#

exact_match=$(git describe --exact-match 2>/dev/null)

if test -z $exact_match; then
	git log -1 --format='g%h'
else
	echo $exact_match |cut -d- -f1 |sed -e 's/^v\([0-9.]*\)/\1/'
fi
