/*
 * Copyright 2021 IBM Corp.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Author(s): Harald Freudenberger <freude@de.ibm.com>
 *
 * s390 zcrypt kubernetes device plugin
 * main function
 */

package main

import (
	"flag"
	"log"
)

func main() {

	// workaround for log: exiting because of error: log cannot create log: open ...
	flag.Set("logtostderr", "true")
	flag.Parse()

	log.Println("Main: S390 k8s z crypto resources plugin starting")

	// check for AP bus support and machine id fetchable or die
	if !apHasApSupport() {
		log.Fatalf("Main: No AP bus support available.\n")
	}
	machineId, err := ccGetMachineId()
	if err != nil {
		log.Fatalf("Main: Reading machine id failed: %s\n", err)
	}
	log.Printf("Main: Machine id is '%s'\n", machineId)

	// initial list of the available apqns on this node or die
	_, err = apScanAPQNs(true)
	if err != nil {
		log.Fatalf("Main: Initial scan of the available APQNs on this node failed: %s\n", err)
	}

	// read the config file or die
	cc, err := ccReadConfigFile()
	if err != nil {
		log.Fatalf("Main: Reading crypto configuration failed: %s\n", err)
	}
	log.Printf("Main: Crypto Configuration successful read\n")
	cc.PrettyLog()
	if !cc.Verify() {
		log.Fatalf("Main: Crypto configuration verification failed.\n")
	}

	// check for zcrypt multiple node support or die
	if !zcryptHasNodesSupport() {
		log.Fatalf("Main: No zcrypt multiple node support available\n")
	}

	// start pod lister or die
	pl := NewPodLister()
	if err = pl.Start(); err != nil {
		log.Fatalf("Main: PodLister Start failed: %s\n", err)
	}

	// enter the crypto resources plugins loop
	RunZCryptoResPlugins(cc)

	// stop pod lister
	pl.Stop()

	log.Println("Main: S390 k8s z crypto resources plugin terminating")
}
