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
 * podlister functions and more
 */

package main

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	podresapi "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	podResSocket = "/var/lib/kubelet/pod-resources/kubelet.sock"
	plConTimeout = 10 // connection timeout
)

var (
	PlPollTime                    = time.Duration(getenvint("PODLISTER_POLL_INTERVAL", 30, 10)) // every plPollTime fetch and process the pod resources
	DeleteResourceTimeoutIfUnused = int64(getenvint("RESOURCE_DELETE_NEVER_USED", 1800, 30))    // delete never used resources after 30min
	DeleteResourceTimeoutAfterUse = int64(getenvint("RESOURCE_DELETE_UNUSED", 120, 30))         // delete not any more used resources after 2 min
)

func getenvstr(envvar, defaultval string) string {
	valstr, isset := os.LookupEnv(envvar)
	if isset {
		return valstr
	}
	return defaultval
}

func getenvint(envvar string, defaultval, minval int) int {
	valstr, isset := os.LookupEnv(envvar)
	if isset {
		valint, err := strconv.Atoi(valstr)
		if err != nil {
			log.Printf("Podlister: Invalid setting for %s: %q.  Using default value...\n", envvar, err)
			return defaultval
		}
		if valint < minval {
			return minval
		}
		return valint
	}
	return defaultval
}

func dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {

	return grpc.Dial(unixSocketPath, grpc.WithInsecure(),
		grpc.WithBlock(), grpc.WithTimeout(timeout),
		grpc.WithDialer(
			func(addr string, timeout time.Duration) (net.Conn, error) {
				return net.DialTimeout("unix", addr, timeout)
			}),
	)
}

type PodLister struct {
	stopChan chan struct{}
	socket   string
	con      *grpc.ClientConn
	client   podresapi.PodResourcesListerClient
}

func NewPodLister() *PodLister {

	return &PodLister{
		socket:   podResSocket,
		stopChan: make(chan struct{}),
	}
}

func (pl *PodLister) connect() error {

	if pl.con != nil {
		pl.con.Close()
		pl.con = nil
	}

	con, err := dial(pl.socket, plConTimeout*time.Second)
	if err != nil {
		log.Printf("PodLister: Socket connection to '%s' failed: %s\n", pl.socket, err)
		return fmt.Errorf("PodLister: Can't establish connection to '%s': %s", pl.socket, err)
	}

	client := podresapi.NewPodResourcesListerClient(con)
	if client == nil {
		pl.con.Close()
		pl.con = nil
		log.Printf("PodLister: NewPodResourcesListerClient() returned nil\n")
		return fmt.Errorf("PodLister: Can't construct pod lister client")
	}

	pl.con = con
	pl.client = client

	return nil
}

func (pl *PodLister) Start() error {

	log.Printf("PodLister: Start()\n")

	err := pl.connect()
	if err != nil {
		log.Printf("PodLister: Unable to construct pod lister client\n")
		return fmt.Errorf("PodLister: Unable to construct pod lister client")
	}

	go pl.podListerLoop()

	return nil
}

func (pl *PodLister) Stop() {

	log.Printf("PodLister: Stop()\n")

	close(pl.stopChan)
	pl.con.Close()
}

func (pl *PodLister) podListerLoop() {

	var err error

	tick := time.NewTicker(PlPollTime * time.Second)

ForLoop:
	for {
		select {
		case <-pl.stopChan:
			tick.Stop()
			break ForLoop
		case <-tick.C:
			err = pl.doLoop()
		}
		if err != nil {
			pl.connect()
		}
	}
}

type zcryptnode_s struct {
	first time.Time // first ever seen timestamp
	last  time.Time // timestamp when last use by a container was seen
}

var zcryptnodemap = map[string]*zcryptnode_s{}

type sysfsshadow_s struct {
	first time.Time // first ever seen timestamp
	last  time.Time // timestamp when last use by a container was seen
}

var sysfsshadowmap = map[string]*sysfsshadow_s{}

func (pl *PodLister) doLoop() error {

	if pl.con == nil {
		log.Printf("PodLister: No connection to kubelet\n")
		return fmt.Errorf("PodLister: No connection to kubelet")
	}

	// update zcryptnodemap with maybe new active zcrypt nodes
	zcryptnodes, err := zcryptFetchActiveNodes()
	if err != nil {
		return nil
	}
	log.Printf("PodLister: %d active zcrypt nodes\n", len(zcryptnodes))
	for _, zn := range zcryptnodes {
		_, found := zcryptnodemap[zn]
		if !found {
			zcryptnodemap[zn] = &zcryptnode_s{
				first: time.Now(),
			}
			log.Printf("PodLister: first time seen zcryptnode '%s' added to zcryptnodemap\n", zn)
		}
	}

	// update sysfsshadowmap with maybe new active shadow sysfs dirs
	shadows, err := shadowFetchActiveShadows()
	if err != nil {
		return nil
	}
	log.Printf("PodLister: %d active sysfs shadow dirs\n", len(shadows))
	for _, sn := range shadows {
		_, found := sysfsshadowmap[sn]
		if !found {
			sysfsshadowmap[sn] = &sysfsshadow_s{
				first: time.Now(),
			}
			log.Printf("PodLister: first time seen sysfsshadow '%s' added to sysfsshadowmap\n", sn)
		}
	}

	// fetch all currently active pods
	req := podresapi.ListPodResourcesRequest{}
	resp, err := pl.client.List(context.TODO(), &req)
	if err != nil {
		log.Printf("PodLister: List() on PodResourcesListerClient failed: %s\n", err)
		return fmt.Errorf("PodLister: List() on PodResourcesListerClient failed: %s", err)
	}

	/* for debugging:
	fmt.Printf("found %d pods:\n", len(resp.PodResources))
	for _, pod := range resp.PodResources {
		fmt.Printf(" pod '%s' namespace '%s' has %d containers:\n", pod.Name, pod.Namespace, len(pod.Containers))
		for _, c := range pod.Containers {
			fmt.Printf("  container '%s' has %d allocated devices\n", c.Name, len(c.Devices))
			if len(c.Devices) > 0 {
				fmt.Printf("   devices:")
				for _, d := range c.Devices {
					fmt.Printf(" ['%s' '%v']", d.ResourceName, d.DeviceIds)
				}
				fmt.Printf("\n")
			}
		}
	}
	*/

	// go through all the active pods and examine the containers which have a device we manage in this plugin
	conswithplugindevs := 0
	for _, pod := range resp.PodResources {
		for _, c := range pod.Containers {
			for _, d := range c.Devices {
				if !strings.HasPrefix(d.ResourceName, baseResourceName+"/") {
					continue
				}
				for _, id := range d.DeviceIds {
					if !strings.HasPrefix(id, "apqn-") {
						continue
					}
					var card, queue, overcount int
					n, err := fmt.Sscanf(id, ApqnFmtStr, &card, &queue, &overcount)
					if err != nil || n < 3 {
						log.Printf("PodLister: Error parsing device id '%s'\n", id)
						continue
					}
					// find the crypto config set to which this apqn belongs
					ccset := GetCurrentCryptoConfig().GetCryptoConfigSetForThisAPQN(card, queue, MachineId)
					if ccset == nil {
						log.Printf("PodLister: config set for APQN(%d,%d) not found\n", card, queue)
					} else {
						// check pod namespace against config set projectname
						if pod.Namespace != ccset.Project {
							log.Printf("PodLister: Container '%s' in namespace '%s' uses CEX resource '%s' marked for project '%s'!!!\n",
								c.Name, pod.Namespace, id, ccset.Project)
						} else {
							log.Printf("PodLister: Container '%s' in namespace %s uses CEX resource '%s'\n",
								c.Name, pod.Namespace, id)
						}
						MetricsCollNotifyAboutRunningContainer(ccset.SetName, id)
					}

					conswithplugindevs++
					// check/update zcryptnodemap
					znname := "zcrypt-" + id
					zn, znfound := zcryptnodemap[znname]
					if znfound {
						zn.last = time.Now()
						//log.Printf("PodLister: last timestamp of zcryptnode '%s' refreshed\n", znname)
					} else {
						log.Printf("PodLister: zcryptnode '%s' not found in zcryptnodemap !!!\n", znname)
					}
					// check/update sysfsshadowmap
					snname := "sysfs-" + id
					sn, snfound := sysfsshadowmap[snname]
					if snfound {
						sn.last = time.Now()
						//log.Printf("PodLister: last timestamp of sysfsshadow '%s' refreshed\n", snname)
					} else {
						log.Printf("PodLister: sysfs shadow '%s' not found in sysfs shadowmap !!!\n", snname)
					}
				}
			}
		}
	}
	log.Printf("PodLister: %d active containers with allocated cex devices\n", conswithplugindevs)

	// go through the zcryptnodemap and check if entries have expired
	for zk, zn := range zcryptnodemap {
		if zn.last.IsZero() {
			dt := time.Since(zn.first).Milliseconds() / 1000
			if dt > DeleteResourceTimeoutIfUnused {
				// within DeleteResourceTimeoutIfUnused s never seen a container using this
				log.Printf("PodLister: deleting zcrypt node '%s': no container ever used it since %d s\n",
					zk, DeleteResourceTimeoutIfUnused)
				pl.tellMetricsCollAboutDestroyNode(zk)
				zcryptDestroyNode(zk)
				delete(zcryptnodemap, zk)
			}
		} else {
			dt := time.Since(zn.last).Milliseconds() / 1000
			if dt > DeleteResourceTimeoutAfterUse {
				// container using this has not been seen for DeleteResourceTimeoutAfterUse s
				log.Printf("PodLister: deleting zcrypt node '%s': no container use since %d s\n",
					zk, DeleteResourceTimeoutAfterUse)
				pl.tellMetricsCollAboutDestroyNode(zk)
				zcryptDestroyNode(zk)
				delete(zcryptnodemap, zk)
			}
		}
	}

	// go through the sysfsshadowmap and check if entries have expired
	for sk, sn := range sysfsshadowmap {
		if sn.last.IsZero() {
			dt := time.Since(sn.first).Milliseconds() / 1000
			if dt > DeleteResourceTimeoutIfUnused {
				// within DeleteResourceTimeoutIfUnused s never seen a container using this
				log.Printf("PodLister: deleting shadow sysfs '%s': no container ever used it since %d s\n",
					sk, DeleteResourceTimeoutIfUnused)
				delShadowSysfs(sk)
				delete(sysfsshadowmap, sk)
			}
		} else {
			dt := time.Since(sn.last).Milliseconds() / 1000
			if dt > DeleteResourceTimeoutAfterUse {
				// container using this has not been seen for DeleteResourceTimeoutAfterUse s
				log.Printf("PodLister: deleting shadow sysfs '%s': no container use since %d s\n",
					sk, DeleteResourceTimeoutAfterUse)
				delShadowSysfs(sk)
				delete(sysfsshadowmap, sk)
			}
		}
	}

	return nil
}

func (pl *PodLister) tellMetricsCollAboutDestroyNode(zcryptnode string) {

	if strings.HasPrefix(zcryptnode, "zcrypt-") {
		dev := zcryptnode[7:]
		MetricsCollNotifyAboutDestroyNode(dev)
	}
}
