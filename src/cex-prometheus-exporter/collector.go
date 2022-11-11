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
 * Metrics data collector server
 */

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

var (
	collPort       = getenvint("COLLECTOR_SERVICE_PORT", 12358, 0) // the metrics collector listener port
	collTCPTimeout = 5 * time.Second                               // read and write timeout for all TCP connections
)

// data structs for the metrics data pushed by the cex plugin apps
type cset_mc_data_s struct {
	Setname          string // cex config set name
	Total_plugindevs int    // total nr of plugin devices in this set
	Used_plugindevs  int    // nr of plugin devices currently in use in this set
	Request_counter  int    // current sum of request counters for all cex resources (APQNs) in this set
}
type mc_data_s struct {
	timestamp        time.Time         // received time
	Nodename         string            // nodename of the cex plugin app
	Total_plugindevs int               // total nr of plugin devices provided
	Used_plugindevs  int               // nr of plugin devices currently in use
	Request_counter  int               // current sum of request couters for all cex resources (APQNs)
	Csets            []*cset_mc_data_s // array holding per cex config set data
}

type MetricsCollector struct {
	li net.Listener
}

func NewMetricsCollector() *MetricsCollector {

	return &MetricsCollector{}
}

func (mc *MetricsCollector) Start() error {

	log.Println("Collector: Start()")

	go mc.loop()

	return nil
}

func (mc *MetricsCollector) Stop() {

	log.Println("Collector: Stop()")

	if mc.li != nil {
		mc.li.Close()
		mc.li = nil
	}
}

func (mc *MetricsCollector) loop() {

	var err error

	mc.li, err = net.Listen("tcp", fmt.Sprintf(":%d", collPort))
	if err != nil {
		log.Fatalf("Collector: Listen on port %d failed: %s\n", collPort, err)
	}
	log.Printf("Collector: Listening on port %d\n", collPort)

	for {
		con, err := mc.li.Accept()
		if err != nil {
			log.Printf("Collector: Accept on port %d failed: %s\n", collPort, err)
			break
		}
		//log.Printf("Collector: new connection from %s\n", con.RemoteAddr())
		go mc.handleConnection(con)
	}

	if mc.li != nil {
		mc.li.Close()
	}
}

func (mc *MetricsCollector) handleConnection(con net.Conn) bool {

	var mcd mc_data_s
	buf := make([]byte, 16*1024)

	defer con.Close()

	con.SetReadDeadline(time.Now().Add(collTCPTimeout))
	n, err := con.Read(buf)
	if err != nil {
		log.Printf("Collector: Connection read error: %s\n", err)
		return false
	}
	if n == len(buf) {
		log.Println("Collector: Receive buffer exceeded !!!")
		return false
	}

	log.Printf("Collector: received %d bytes from client %s\n", n, con.RemoteAddr())

	if err := json.Unmarshal(buf[0:n], &mcd); err != nil {
		log.Printf("Collector: Error parsing raw metrics data from client %s: %s\n", con.RemoteAddr(), err)
		return false
	}
	mcd.timestamp = time.Now()

	con.SetWriteDeadline(time.Now().Add(collTCPTimeout))
	if _, err = con.Write([]byte("ok\n")); err != nil {
		log.Printf("Collector: Connection write error: %s\n", err)
		return false
	}

	// for debugging:
	//fmt.Printf("Collector: metrics data from client %s:\n", con.RemoteAddr())
	//fmt.Printf("Collector:   Nodename '%s' Total_plugindevs %d Used_plugindevs %d Request_counter %d\n",
	//	mcd.Nodename, mcd.Total_plugindevs, mcd.Used_plugindevs, mcd.Request_counter)
	//fmt.Printf("Collector:   and %d config set metrics data:\n", len(mcd.Csets))
	//for _, mcs := range mcd.Csets {
	//	fmt.Printf("Collector:     Setname '%s' Total_plugindevs %d Used_plugindevs %d Request_counter %d\n",
	//		mcs.Setname, mcs.Total_plugindevs, mcs.Used_plugindevs, mcs.Request_counter)
	//}

	raddr := con.RemoteAddr().String()
	ipaddr := raddr[0:strings.LastIndexByte(raddr, ':')]
	dpStoreNodeMetricsData(ipaddr, &mcd)

	return true
}
