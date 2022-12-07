/*
 * Copyright 2022 IBM Corp.
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
 * Prometheus exporter for the s390 zcrypt kubernetes device plugin
 * Main function
 */

package main

import (
	"flag"
	//"fmt"
	"log"
	"os"
	"strconv"
)

var (
	version    = "development"
	git_url    = "https://github.com/ibm-s390-cloud/k8s-cex-dev-plugin.git"
	git_commit = "unknown"
)

func getenvint(envvar string, defaultval, minval int) int {
	valstr, isset := os.LookupEnv(envvar)
	if isset {
		valint, err := strconv.Atoi(valstr)
		if err != nil {
			log.Printf("Main: Invalid setting for %s: %q.  Using default value...\n", envvar, err)
			return defaultval
		}
		if valint < minval {
			return minval
		}
		return valint
	}
	return defaultval
}

func main() {

	versionarg := flag.Bool("version", false, "Print version and exit")

	// workaround for log: exiting because of error: log cannot create log: open ...
	flag.Set("logtostderr", "true")
	flag.Parse()

	log.Println("Main: S390 k8s cex plugin prometheus exporter")
	log.Printf("Plugin Version: %s\n", version)
	log.Printf("Git URL:        %s\n", git_url)
	log.Printf("Git Commit:     %s\n", git_commit)

	// exit if only version was requested
	if *versionarg {
		os.Exit(0)
	}

	// start metrics data collector server
	mc := NewMetricsCollector()
	if err := mc.Start(); err != nil {
		log.Fatalf("Main: MetricsCollector Start failed: %s\n", err)
	}

	// run the prometheus api loop
	promLoop()

	// stop metrics data collector server
	mc.Stop()

	log.Println("Main: S390 k8s cex plugin prometheus exporter terminating")
}

// TODO:
// - provide nr of cex plugings running
// - use timestamp of the node data
