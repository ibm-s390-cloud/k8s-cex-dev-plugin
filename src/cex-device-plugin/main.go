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
	"os"
)

var (
	version    = "development"
	git_url    = "https://github.com/ibm-s390-cloud/k8s-cex-dev-plugin.git"
	git_commit = "unknown"

	MachineId = ""
)

func main() {

	versionarg := flag.Bool("version", false, "Print version and exit")

	// workaround for log: exiting because of error: log cannot create log: open ...
	flag.Set("logtostderr", "true")
	flag.Parse()

	log.Println("Main: S390 k8s z crypto resources plugin starting")
	log.Printf("Plugin Version: %s\n", version)
	log.Printf("Git URL:        %s\n", git_url)
	log.Printf("Git Commit:     %s\n", git_commit)

	// exit if only version was requested
	if *versionarg {
		os.Exit(0)
	}

	// check for AP bus support and machine id fetchable or die
	if !apHasApSupport() {
		log.Fatalf("Main: No AP bus support available.\n")
	}
	mid, err := ccGetMachineId()
	if err != nil {
		log.Fatalf("Main: Reading machine id failed: %s\n", err)
	}
	MachineId = mid
	log.Printf("Main: Machine id is '%s'\n", MachineId)

	// initial list of the available apqns on this node or die
	_, err = apScanAPQNs(true)
	if err != nil {
		log.Fatalf("Main: Initial scan of the available APQNs on this node failed: %s\n", err)
	}

	// read the config file or die
	cc, err := InitializeConfigWatcher()
	if err != nil {
		log.Fatalf("Main: Reading crypto configuration failed: %s\n", err)
	}
	if cc == nil {
		log.Fatalf("Main: Failed to read crypto configuration\n")
	}
	log.Printf("Main: Crypto configuration successful read\n")
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

	// start metrics collector or die
	mc := NewMetricsCollector()
	if err = mc.Start(); err != nil {
		log.Fatalf("Main: MetricsCollector Start failed: %s\n", err)
	}

	// enter the crypto resources plugins loop
	RunZCryptoResPlugins()

	// stop metrics collector
	mc.Stop()

	// stop pod lister
	pl.Stop()

	// stop the config watcher
	StopConfigWatcher()

	log.Println("Main: S390 k8s z crypto resources plugin terminating")
}
