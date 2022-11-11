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
 * s390 zcrypt kubernetes device plugin
 * Metrics collector. Provides an interface to the other device plugin
 * components for reporting relevant events and data as raw data for
 * some metrics.
 */

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	mcPollTime                = time.Duration(getenvint("METRICS_POLL_INTERVAL", 15, 10))
	promExporterCollService   = getenvstr("CEX_PROM_EXPORTER_COLLECTOR_SERVICE", "cex-prometheus-exporter-collector-service")
	promExporterCollNamespace = getenvstr("CEX_PROM_EXPORTER_COLLECTOR_SERVICE_NAMESPACE", "")
	promExporterCollPort      = getenvint("CEX_PROM_EXPORTER_COLLECTOR_SERVICE_PORT", 12358, 0)
	conTCPTimeout             = 5 * time.Second // connect, read and write timeout for the TCP connection
)

type plugindev_entry_s struct {
	in_use  bool
	trcseen time.Time // time of last running container notification
}
type apqn_entry_s struct {
	start_request_count   int // apqn's start request_count value
	current_request_count int // current apqns's request_count
}
type cset_entry_s struct {
	plugindevs map[string]*plugindev_entry_s
	apqns      map[int]*apqn_entry_s // int key here holds dom and ap: dom = key % 256, ap = key / 256
}

var csetmap = map[string]*cset_entry_s{}
var mcmutex = sync.Mutex{}

func dumpRawMetricsData() {

	fmt.Printf("MetricsColl: %d cset entries:\n", len(csetmap))
	for sn, cse := range csetmap {
		fmt.Printf("  setname '%s':\n", sn)
		fmt.Printf("    %d plugin devs:\n", len(cse.plugindevs))
		for dev, pde := range cse.plugindevs {
			fmt.Printf("      '%s': in_use %t\n", dev, pde.in_use)
		}
		fmt.Printf("    %d apqns:\n", len(cse.apqns))
		for k, ae := range cse.apqns {
			fmt.Printf("      APQN(%d,%d): start count: %d current count: %d\n",
				k/256, k%256, ae.start_request_count, ae.current_request_count)
		}
	}
}

func MetricsCollNotifyAboutAlloc(setname, dev string) {

	log.Printf("MetricsColl: Alloc notify, setname=%s dev=%s\n", setname, dev)

	var ap, dom, overcount int
	n, err := fmt.Sscanf(dev, ApqnFmtStr, &ap, &dom, &overcount)
	if err != nil || n < 3 {
		log.Printf("MetricsColl: Error parsing plugin device '%s'\n", dev)
		return
	}

	mcmutex.Lock()
	defer mcmutex.Unlock()

	cse, found := csetmap[setname]
	if !found {
		// Allocation notify for a unknown config set, this should not happen
		log.Printf("MetricsColl: Alloc notify for setname=%s but no set data entry found\n", setname)
		return
	}
	pde, found := cse.plugindevs[dev]
	if !found {
		// Allocation notify for a unknown plugin device, this should not happen
		log.Printf("MetricsColl: Alloc notify for setname=%s with unknown dev=%s\n", setname, dev)
		return
	}
	pde.in_use = true
	pde.trcseen = time.Now()

	//dumpRawMetricsData()
}

func MetricsCollNotifyAboutDestroyNode(dev string) {

	log.Printf("MetricsColl: DestroyNode notify, dev=%s\n", dev)

	mcmutex.Lock()
	defer mcmutex.Unlock()

	for _, cse := range csetmap {
		if pde, found := cse.plugindevs[dev]; found {
			pde.in_use = false
			break
		}
	}

	//dumpRawMetricsData()
}

func MetricsCollAPQNs(setname string, apqns APQNList) {

	log.Printf("MetricsColl: APQNs notify, setname=%s apqns=%v\n", setname, apqns)

	mcmutex.Lock()
	defer mcmutex.Unlock()

	// search for an existing entry for this config set, maybe add a new one
	cse, found := csetmap[setname]
	if !found {
		// alloc a new config set entry
		csetmap[setname] = &cset_entry_s{
			plugindevs: make(map[string]*plugindev_entry_s),
			apqns:      make(map[int]*apqn_entry_s),
		}
		cse = csetmap[setname]
	}

	// update the APQN entries within this config set entry
	// 1. delete all APQN entries which are not present any more
	for k, _ := range cse.apqns {
		ap := k / 256
		dom := k % 256
		found = false
		for _, a := range apqns {
			if ap == a.Adapter && dom == a.Domain {
				found = true
				break
			}
		}
		if !found {
			delete(cse.apqns, k)
		}
	}
	// 2. add all APQNs which are not yet in the list
	for _, a := range apqns {
		k := (256 * a.Adapter) + a.Domain
		if _, found = cse.apqns[k]; !found {
			cse.apqns[k] = &apqn_entry_s{}
			ae := cse.apqns[k]
			ae.start_request_count, _ = apGetQueueRequestCounter(k/256, k%256)
		}
	}

	//dumpRawMetricsData()
}

func MetricsCollPluginDevs(setname string, devs []string) {

	log.Printf("MetricsColl: PluginDevs notify, setname=%s devs=%v\n", setname, devs)

	mcmutex.Lock()
	defer mcmutex.Unlock()

	// search for an existing entry for this config set, maybe add a new one
	cse, found := csetmap[setname]
	if !found {
		// alloc a new config set entry
		csetmap[setname] = &cset_entry_s{
			plugindevs: make(map[string]*plugindev_entry_s),
			apqns:      make(map[int]*apqn_entry_s),
		}
		cse = csetmap[setname]
	}

	// update the plugin device entries within this config set entry
	// 1. delete all plugin device entries which are not present any more
	for k, _ := range cse.plugindevs {
		found = false
		for _, d := range devs {
			if k == d {
				found = true
				break
			}
		}
		if !found {
			delete(cse.plugindevs, k)
		}
	}
	// 2. add all plugin devices which are not yet in the list
	for _, d := range devs {
		if _, found = cse.plugindevs[d]; !found {
			cse.plugindevs[d] = &plugindev_entry_s{}
		}
	}

	//dumpRawMetricsData()
}

func MetricsCollNotifyAboutRunningContainer(setname, dev string) {

	log.Printf("MetricsColl: Container Running notify, setname=%s dev=%s\n", setname, dev)

	var ap, dom, overcount int
	n, err := fmt.Sscanf(dev, ApqnFmtStr, &ap, &dom, &overcount)
	if err != nil || n < 3 {
		log.Printf("MetricsColl: Error parsing plugin device '%s'\n", dev)
		return
	}

	mcmutex.Lock()
	defer mcmutex.Unlock()

	cse, found := csetmap[setname]
	if !found {
		log.Printf("MetricsColl: Container Running notify for setname=%s but no set data entry found\n", setname)
		return
	}
	pde, found := cse.plugindevs[dev]
	if !found {
		log.Printf("MetricsColl: Container Running notify for setname=%s with unknown dev=%s\n", setname, dev)
		return
	}
	pde.in_use = true
	pde.trcseen = time.Now()

	//dumpRawMetricsData()
}

type MetricsCollector struct {
	stopChan chan struct{}
	nodename string
}

func NewMetricsCollector() *MetricsCollector {

	nn, found := os.LookupEnv("NODENAME")

	if !found {
		log.Fatalf("MetricsColl: Missing NODENAME env setting.\n")
	}

	return &MetricsCollector{
		stopChan: make(chan struct{}),
		nodename: nn,
	}
}

func (mc *MetricsCollector) Start() error {

	log.Printf("MetricsColl: Start()\n")

	go mc.Loop()
	return nil
}

func (mc *MetricsCollector) Stop() {

	log.Printf("MetricsColl: Stop()\n")

	close(mc.stopChan)
}

func (mc *MetricsCollector) Loop() {

	tick := time.NewTicker(mcPollTime * time.Second)

ForLoop:
	for {
		select {
		case <-mc.stopChan:
			tick.Stop()
			break ForLoop
		case <-tick.C:
			mc.doLoop()
		}
	}
}

// per config set struct for the data sent to cex prometheus exporter collector
type cset_pe_data_s struct {
	Setname          string
	Total_plugindevs int
	Used_plugindevs  int
	Request_counter  int
}

// per cex plugin app struct for the data sent to cex prometheus exporter collector
type pe_data_s struct {
	Nodename         string
	Total_plugindevs int
	Used_plugindevs  int
	Request_counter  int
	Csets            []*cset_pe_data_s
}

func (mc *MetricsCollector) doLoop() {

	//log.Printf("MetricsColl: doLoop()\n")

	mcmutex.Lock()

	// fetch latest APQN request counts for all config sets
	for _, cse := range csetmap {
		// a plugin device where no container has been seen for
		// more than 2 * PodLister Polltime, is not in use any more
		nowminus2xPollTime := time.Now().Add(time.Duration(-2) * PlPollTime * time.Second)
		for _, pde := range cse.plugindevs {
			if pde.in_use && pde.trcseen.Before(nowminus2xPollTime) {
				pde.in_use = false
			}
		}
		// fetch the current request count value for all APQNs
		for k, ae := range cse.apqns {
			ae.current_request_count, _ = apGetQueueRequestCounter(k/256, k%256)
			if ae.start_request_count == 0 {
				ae.start_request_count = ae.current_request_count
			}
		}
	}

	//dumpRawMetricsData()

	// accumulate the raw metrics into the send data struct
	senddata := mc.prepPromExpData()

	// unlock the raw data metrics lock
	mcmutex.Unlock()

	// send the prepared data to the cex prometheus exporter
	mc.sendDataToPromExp(senddata)
}

func (mc *MetricsCollector) prepPromExpData() *pe_data_s {

	// accumulate the raw metrics data into a new data
	// struct, the mcmutex is locked by the caller

	pe_data := &pe_data_s{
		Nodename: mc.nodename,
	}
	var cset_pe_data []*cset_pe_data_s
	for sn, cse := range csetmap {
		cspe := &cset_pe_data_s{}
		cspe.Setname = sn
		cspe.Total_plugindevs = len(cse.plugindevs)
		pe_data.Total_plugindevs += cspe.Total_plugindevs
		for _, pe := range cse.plugindevs {
			if pe.in_use {
				cspe.Used_plugindevs++
				pe_data.Used_plugindevs++
			}
		}
		for _, ae := range cse.apqns {
			cspe.Request_counter += ae.current_request_count - ae.start_request_count
			pe_data.Request_counter += cspe.Request_counter
		}
		cset_pe_data = append(cset_pe_data, cspe)
	}
	pe_data.Csets = cset_pe_data

	return pe_data
}

func (mc *MetricsCollector) sendDataToPromExp(senddata *pe_data_s) bool {

	var addr string

	// serialize the data into a json stream
	data, err := json.Marshal(senddata)
	if err != nil {
		log.Printf("MetricsColl: data marshal error: %v\n", err)
		return false
	}

	// for debugging:
	//s := string(data)
	//fmt.Printf("MetricsColl: marshalled data: '%v'\n", s)

	// TCP connect to exporter
	if len(promExporterCollNamespace) > 0 {
		addr = fmt.Sprintf("%s.%s:%d", promExporterCollService,
			promExporterCollNamespace, promExporterCollPort)
	} else {
		addr = fmt.Sprintf("%s:%d", promExporterCollService, promExporterCollPort)
	}
	con, err := net.DialTimeout("tcp", addr, conTCPTimeout)
	if err != nil {
		log.Printf("MetricsColl: Connection to '%s' failed: %s\n", addr, err)
		return false
	}
	defer con.Close()

	// send the json data
	con.SetWriteDeadline(time.Now().Add(conTCPTimeout))
	_, err = con.Write(data)
	if err != nil {
		log.Printf("MetricsColl: Connection write error: %v\n", err)
		return false
	}

	// receive ack or or error string
	buf := make([]byte, 1024)
	con.SetReadDeadline(time.Now().Add(conTCPTimeout))
	_, err = con.Read(buf)
	if err != nil {
		log.Printf("MetricsColl: Connection read error: %v\n", err)
		return false
	}
	i := bytes.IndexByte(buf, 0x0a)
	if i < 1 {
		log.Printf("MetricsColl: Connection received invalid reply\n")
		return false
	}
	str := strings.TrimSpace(string(buf[:i]))
	if str != "ok" {
		log.Printf("MetricsColl: Connection received invalid reply '%s'\n", str)
		return false
	}

	log.Printf("MetricsColl: %d bytes metrics data pushed successful to cex-prometheus-exporter\n", len(data))

	return true
}
