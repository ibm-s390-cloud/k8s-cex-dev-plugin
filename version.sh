#!/bin/bash
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
# Script to get version information for the IBM Cex Device Plugin
# Author(s): Jan Schintag <jan.schintag@de.ibm.com>

usage_info () {
	echo "This script provides version information for the ci"
	echo "Usage:"
	echo "-v, --version   : Print version information."
	echo "                  Optional: -v nightly (Example output: v1.0.0-2021-09-09-134200)"
	echo "-r, --release   : Print release number"
	echo "--git-url       : Print URL of the Git Repository"
	echo "--git-commit    : Print Git Commit ID"
	echo "-h, --help      : Show this information"
}

print_version () {
	nightly=""
	if [ "$1" == "nightly" ]; then
		nightly="-$(date +%Y-%m-%d-%H%M%S)"
	fi
	version="$(jq -r .version version.json)"
	printf "${version}${nightly}"
}

print_release () {
	version="$(print_version)"
	IFS="."
	release=($version)
	unset IFS
	printf "${release[2]}"
}

print_git_url () {
	git_ref="$(git symbolic-ref -q HEAD)"
	git_remote="$(git for-each-ref --format='%(upstream:remotename)' ${git_ref})"
	git_url="$(git remote get-url ${git_remote})"
	printf "${git_url}"
}

print_git_commit () {
	git_commit="$(git rev-parse HEAD)"
	printf "${git_commit}"
}

case $1 in
	-v | --version )
		shift
		print_version $1
		shift
		;;
	-r | --release )
		shift
		print_release
		;;
	--git-url )
		shift
		print_git_url
		;;
	--git-commit )
		shift
		print_git_commit
		;;
	-h | --help )
		shift
		usage_info
		;;
	* )
		shift
		echo "Error: Unkown Command" >&2
		usage_info
		exit 1
		;;
esac
