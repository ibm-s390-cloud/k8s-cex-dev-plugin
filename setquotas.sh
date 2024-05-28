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
# Author(s): Juergen Christ <jchrist@linux.ibm.com>
#
# This script produces a yaml file with quota restrictions
# for the cex cryptosets for each given namespace.
# Apply the yaml file and then only the namespace <xxx>
# is allowed  to allocate CEX resources from a crypto
# set marked with project <xxx>.

createquota () {
    QF=quota-$1.yaml
    cat << EOF >> $QF
- apiVersion: v1
  kind: ResourceQuota
  metadata:
    name: cex.$3
    namespace: $1
  spec:
    hard:
      requests.cex.s390.ibm.com/$2: 0
      limits.cex.s390.ibm.com/$2: 0
EOF
}

while ! test -z "$1"; do
    n=$1
    shift
    c=0
    echo "apiVersion: v1" > quota-$n.yaml
    echo "items:" >> quota-$n.yaml
    for s in `oc get cm cex-resources-config -n cex-device-plugin -o jsonpath='{.data.cex_resources\.json}' | jq -r ".cryptoconfigsets | .[] | select(.project != \"$n\") | .setname"`; do
	c=$(( c + 1 ))
	createquota $n $s $c
    done
    echo "kind: List" >> quota-$n.yaml
    echo "metadata: {}" >> quota-$n.yaml
    ## TODO: apply it
done
