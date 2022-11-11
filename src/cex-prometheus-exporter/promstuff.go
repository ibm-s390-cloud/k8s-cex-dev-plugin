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
 * This file implements the Prometheus stuff needed to act as a
 * data source for Prometheus.
 */

package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"time"
)

var (
	promPort                 = getenvint("PROMETHEUS_SERVICE_PORT", 9939, 0) // the prometheus client port
	promGaugesUpdateInterval = 10 * time.Second                              // update interval in s for the Prom Gauges
)

func promLoop() {

	tlast := time.Now()

	// create and register all the prometheus objects needed
	total_plugindevs_available := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: "cex_plugin",
			Name:      "total_plugindevs_available",
			Help:      "Total number of CEX plugin devices available",
		})
	prometheus.MustRegister(total_plugindevs_available)
	log.Println("Promstuff: Prometheus Gauge cex_plugin_total_plugindevs_available created")
	total_plugindevs_used := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: "cex_plugin",
			Name:      "total_plugindevs_used",
			Help:      "Total number of CEX plugin devices in use",
		})
	prometheus.MustRegister(total_plugindevs_used)
	log.Println("Promstuff: Prometheus Gauge cex_plugin_total_plugindevs_used created")
	total_request_counter := prometheus.NewGauge(
		prometheus.GaugeOpts{
			Subsystem: "cex_plugin",
			Name:      "total_request_counter",
			Help:      "Sum of all request counter values of all CEX resources managed by all CEX plugins",
		})
	prometheus.MustRegister(total_request_counter)
	log.Println("Promstuff: Prometheus Gauge cex_plugin_total_request_counter created")
	plugindevs_available := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "cex_plugin",
			Name:      "plugindevs_available",
			Help:      "Number of CEX plugin devices available, partitioned by configset",
		},
		[]string{"setname"},
	)
	prometheus.MustRegister(plugindevs_available)
	log.Println("Promstuff: Prometheus GaugeVec cex_plugin_plugindevs_available created")
	plugindevs_used := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "cex_plugin",
			Name:      "plugindevs_used",
			Help:      "Number of CEX plugin devices in use, partitioned by configset",
		},
		[]string{"setname"},
	)
	prometheus.MustRegister(plugindevs_used)
	log.Println("Promstuff: Prometheus GaugeVec cex_plugin_plugindevs_used created")
	request_counter := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Subsystem: "cex_plugin",
			Name:      "request_counter",
			Help:      "Sum of request counter values of all CEX resources managed by all CEX plugins, partitioned by configset",
		},
		[]string{"setname"},
	)
	prometheus.MustRegister(request_counter)
	log.Println("Promstuff: Prometheus GaugeVec cex_plugin_request_counter created")

	// start the prometheus metrics http interface
	http.Handle("/metrics", promhttp.Handler())
	listenandservefunc := func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", promPort), nil)
		if err != nil {
			log.Fatalf("Promstuff: http server error: %s\n", err)
		}
	}
	go listenandservefunc()

	// loop and update the prometheus objects
	for {
		Cluster_mc_data_mutex.Lock()
		if Cluster_mc_data == nil {
			Cluster_mc_data_mutex.Unlock()
			continue
		}
		total_plugindevs_available.Set(float64(Cluster_mc_data.Total_plugindevs))
		total_plugindevs_used.Set(float64(Cluster_mc_data.Used_plugindevs))
		total_request_counter.Set(float64(Cluster_mc_data.Request_counter))
		if tlast.Add(10 * time.Minute).Before(time.Now()) {
			plugindevs_available.Reset()
			plugindevs_used.Reset()
			request_counter.Reset()
		}
		for _, cs := range Cluster_mc_data.Cset_mc_data {
			sn := cs.Setname
			plugindevs_available.WithLabelValues(sn).Set(float64(cs.Total_plugindevs))
			plugindevs_used.WithLabelValues(sn).Set(float64(cs.Used_plugindevs))
			request_counter.WithLabelValues(sn).Set(float64(cs.Request_counter))
		}
		Cluster_mc_data_mutex.Unlock()
		time.Sleep(promGaugesUpdateInterval)
	}
}
