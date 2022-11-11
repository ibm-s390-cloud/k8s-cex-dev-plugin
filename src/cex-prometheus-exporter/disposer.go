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
 * gather and manage Metrics data
 */

package main

import (
	//"fmt"
	"log"
	"sync"
	"time"
)

// data structs to hold the accumulated metrics data from the cex plugin apps
type cluster_mc_data_s struct {
	Total_plugindevs int               // total nr of plugin devices provided
	Used_plugindevs  int               // nr of plugin devices currently in use
	Request_counter  int               // current sum of request couters for all cex resources (APQNs)
	Cset_mc_data     []*cset_mc_data_s // slice holding per cex config set data
}

var (
	Cluster_mc_data       *cluster_mc_data_s
	Cluster_mc_data_mutex = sync.Mutex{}
)

func dumpClusterMcData(cmc *cluster_mc_data_s) {

	log.Printf("Disposer: Cluster metrics data:\n")
	log.Printf("Disposer:   Total_plugindevs %d Used_plugindevs %d Request_counter %d\n",
		cmc.Total_plugindevs, cmc.Used_plugindevs, cmc.Request_counter)
	for _, cs := range cmc.Cset_mc_data {
		log.Printf("Disposer:     Setname '%s' Total_plugindevs %d Used_plugindevs %d Request_counter %d\n",
			cs.Setname, cs.Total_plugindevs, cs.Used_plugindevs, cs.Request_counter)
	}
}

// latest metrics data per node (ipaddr)
var (
	node_mc_data       = map[string]*mc_data_s{}
	node_mc_data_mutex = sync.Mutex{}
)

func dumpMcData(msg string, mcd *mc_data_s) {

	log.Printf("Disposer: %s\n", msg)
	log.Printf("Disposer:   Nodename '%s' Total_plugindevs %d Used_plugindevs %d Request_counter %d\n",
		mcd.Nodename, mcd.Total_plugindevs, mcd.Used_plugindevs, mcd.Request_counter)
	for _, cs := range mcd.Csets {
		log.Printf("Disposer:     Setname '%s' Total_plugindevs %d Used_plugindevs %d Request_counter %d\n",
			cs.Setname, cs.Total_plugindevs, cs.Used_plugindevs, cs.Request_counter)
	}
}

func updateClusterMcData() {

	var cmc *cluster_mc_data_s = new(cluster_mc_data_s)
	var cset []*cset_mc_data_s

	node_mc_data_mutex.Lock()
	// purge mc data for nodes with timestamp older than 60s
	for k, mcd := range node_mc_data {
		if mcd.timestamp.Add(60 * time.Second).Before(time.Now()) {
			delete(node_mc_data, k)
		}
	}
	// add mc data for the remaining nodes to the cluster metrics data
	for _, mcd := range node_mc_data {
		for _, cs := range mcd.Csets {
			found := false
			var s *cset_mc_data_s
			for _, s = range cset {
				if s.Setname == cs.Setname {
					found = true
					break
				}
			}
			if !found {
				s = new(cset_mc_data_s)
				s.Setname = cs.Setname
				cset = append(cset, s)
			}
			s.Total_plugindevs += cs.Total_plugindevs
			s.Used_plugindevs += cs.Used_plugindevs
			s.Request_counter += cs.Request_counter
		}
	}
	node_mc_data_mutex.Unlock()

	cmc.Cset_mc_data = cset
	for _, cs := range cmc.Cset_mc_data {
		cmc.Total_plugindevs += cs.Total_plugindevs
		cmc.Used_plugindevs += cs.Used_plugindevs
		cmc.Request_counter += cs.Request_counter
	}

	Cluster_mc_data_mutex.Lock()
	// nice to have in the log but not required:
	if Cluster_mc_data == nil ||
		Cluster_mc_data.Total_plugindevs != cmc.Total_plugindevs ||
		Cluster_mc_data.Used_plugindevs != cmc.Used_plugindevs ||
		Cluster_mc_data.Request_counter != cmc.Request_counter ||
		len(Cluster_mc_data.Cset_mc_data) != len(cmc.Cset_mc_data) {
		dumpClusterMcData(cmc)
	}
	Cluster_mc_data = cmc
	Cluster_mc_data_mutex.Unlock()
}

func dpStoreNodeMetricsData(ipaddr string, mcd *mc_data_s) {

	log.Printf("Disposer: new metrics data from %s ('%s')\n", ipaddr, mcd.Nodename)

	// dumpMcData(fmt.Sprintf("mc data from %s", ipaddr), mcd)

	node_mc_data_mutex.Lock()
	node_mc_data[ipaddr] = mcd
	node_mc_data_mutex.Unlock()

	updateClusterMcData()
}
