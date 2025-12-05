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
 * kubernetes device plugin manager implementation
 */

package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	kdp "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	baseResourceName = "cex.s390.ibm.com" // base resource name
	ApqnFmtStr       = "apqn-%d-%d-%d"    // used on several places for Sscanf and Sprintf
)

var (
	apqnLiveSysfs       = getenvint("APQN_LIVE_SYSFS", 1, 0, 1)                        // live sysfs is by default enabled
	apqnOverCommitLimit = getenvint("APQN_OVERCOMMIT_LIMIT", 1, 1, 100)                // overcommit limit: 1 is no overcommit
	apqnsCheckInterval  = time.Duration(getenvint("APQN_CHECK_INTERVAL", 30, 10, 120)) // device health check interval in seconds
)

type ZCryptoDPMLister struct {
	machineid    string
	setnameslist []string
}

type ZCryptoResPlugin struct {
	resource    string
	lister      *ZCryptoDPMLister
	ccset       *CryptoConfigSet
	tag         []byte
	apqns       APQNList
	devices     []*kdp.Device
	stopChan    chan struct{}
	changedChan chan struct{}
	wgChChan    sync.WaitGroup // a wait group for the changed channel
}

func (l *ZCryptoDPMLister) GetResourceNamespace() string {

	log.Printf("Plugin: Announcing '%s' as our resource namespace\n", baseResourceName)

	return baseResourceName
}

func (z *ZCryptoDPMLister) Discover(nameslistchan chan dpm.PluginNameList) {

	areTheseSortedStringListsEqual := func(l1, l2 []string) bool {
		if len(l1) != len(l2) {
			return false
		}
		for i, _ := range l1 {
			if l1[i] != l2[i] {
				return false
			}
		}
		return true
	}

	// prepare and announce the initial list of crypto config setnames
	sets := GetCurrentCryptoConfig().GetListOfSetNames()
	sort.Strings(sets)
	z.setnameslist = sets
	log.Printf("Plugin: Register plugins for these CryptoConfigSets: %v\n", z.setnameslist)
	nameslistchan <- dpm.PluginNameList(z.setnameslist)

	// every Cccheckinterval seconds check if the list of setnames has changed
	tick := time.NewTicker(Cccheckinterval * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-nameslistchan:
			return
		case t := <-tick.C:
			if t.IsZero() {
				return
			}
			sets = GetCurrentCryptoConfig().GetListOfSetNames()
			sort.Strings(sets)
			if !areTheseSortedStringListsEqual(sets, z.setnameslist) {
				z.setnameslist = sets
				log.Printf("Plugin: Found crypto config set changes. Reannouncing: %v\n", z.setnameslist)
				nameslistchan <- dpm.PluginNameList(z.setnameslist)
			} else if len(z.setnameslist) == 0 {
				log.Printf("Plugin: No crypto config sets available, check configuration !\n")
			}
		}
	}
}

func (z *ZCryptoDPMLister) NewPlugin(resource string) dpm.PluginInterface {

	log.Printf("Plugin: NewPlugin('%s')\n", resource)

	ccset, tag := GetCurrentCryptoConfigSet(nil, resource, nil)

	p := &ZCryptoResPlugin{
		lister:   z,
		resource: resource,
		ccset:    ccset,
		tag:      tag,
		wgChChan: sync.WaitGroup{},
	}
	return p
}

func (p *ZCryptoResPlugin) filterAPQNs(ccset *CryptoConfigSet, apqnlist APQNList) APQNList {
	var apqns APQNList
	if ccset == nil {
		return apqns
	}

	for _, a := range apqnlist {
		for _, c := range ccset.APQNDefs {
			if a.Adapter != c.Adapter || a.Domain != c.Domain {
				continue
			}
			if len(c.MachineId) > 0 && p.lister.machineid != c.MachineId {
				continue
			}
			if len(ccset.MinCexGen) > 0 && a.Gen < ccset.MinCexGen {
				log.Printf("Plugin['%s']: APQN (%d,%d) not announced. Card generation = %s, but %s or higher required for this config set\n",
					p.resource, a.Adapter, a.Domain, a.Gen, ccset.MinCexGen)
				continue
			}
			apqns = append(apqns, a)
		}
	}

	return apqns
}

func (p *ZCryptoResPlugin) makePluginDevsFromAPQNs() []*kdp.Device {

	var devices []*kdp.Device

	// if the configset is empty, simple return an empty list
	if p.ccset == nil {
		return devices
	}

	for _, a := range p.apqns {
		health := kdp.Healthy
		if !a.Online {
			health = kdp.Unhealthy
		}
		for i := 0; i < max(1, p.ccset.Overcommit); i++ {
			devices = append(devices, &kdp.Device{
				ID:     fmt.Sprintf(ApqnFmtStr, a.Adapter, a.Domain, i),
				Health: health,
			})
		}
	}

	return devices
}

func (p *ZCryptoResPlugin) checkChanged() bool {

	//log.Printf("Plugin['%s']: checkChanged() rescanning available APQNs\n", p.resource)

	var apqnsChanged, configChanged bool
	ccset, tag := GetCurrentCryptoConfigSet(p.ccset, p.resource, p.tag) // caution: ccset may be nil

	allnodeapqns, err := apScanAPQNs(false)
	if err != nil {
		log.Printf("Plugin['%s']: failure trying to rescan node APQNs: %s\n", p.resource, err)
		return false
	}

	// check for change in APQNs
	apqns := p.filterAPQNs(ccset, allnodeapqns)
	if !apEqualAPQNLists(apqns, p.apqns) {
		log.Printf("Plugin['%s']: Rescan found %d eligible APQNs (with changes): %s\n",
			p.resource, len(apqns), apqns)
		apqnsChanged = true
	}

	// adjust the ConfigSet before comparing
	if ccset != nil {
		if ccset.Overcommit < 0 {
			// no overcommit parameter given in this config set, so use default
			ccset.Overcommit = apqnOverCommitLimit
		}
		if ccset.Livesysfs < 0 {
			// no livesysfs parameter given in this config set, so use default
			ccset.Livesysfs = apqnLiveSysfs
		}
	}

	// check for overcommit change in ConfigSet
	if ccset != nil && ccset.Overcommit != p.ccset.Overcommit {
		log.Printf("Plugin['%s']: Rescan found changes in ConfigSet: overcommit limit has changed\n", p.resource)
		configChanged = true
	}

	// check for livesysfs change in ConfigSet
	if ccset != nil && ccset.Livesysfs != p.ccset.Livesysfs {
		log.Printf("Plugin['%s']: Rescan found changes in ConfigSet: livesysfs parameter has changed\n", p.resource)
		configChanged = true
	}

	if apqnsChanged || configChanged {
		p.ccset, p.tag = ccset, tag
		p.apqns = apqns
		p.tellMetricsCollAboutAPQNs()
		p.devices = p.makePluginDevsFromAPQNs()
		log.Printf("Plugin['%s']: Derived %d plugin devices from the list of APQNs\n",
			p.resource, len(p.devices))
		p.tellMetricsCollAboutPluginDevs()
		return true
	} else {
		log.Printf("Plugin['%s']: no changes\n", p.resource)
		return false
	}
}

func (p *ZCryptoResPlugin) checkChangedLoop() {

	tick := time.NewTicker(apqnsCheckInterval * time.Second)

	// add one user (the for loop we will run into now) to the wait group
	p.wgChChan.Add(1)

ForLoop:
	for {
		select {
		case <-p.stopChan:
			tick.Stop()
			break ForLoop
		case <-tick.C:
			if p.checkChanged() {
				p.changedChan <- struct{}{}
			}
		}
	}

	// release our usage of the changed channel
	p.wgChChan.Done()
}

func (p *ZCryptoResPlugin) Start() error {

	log.Printf("Plugin['%s']: Start()\n", p.resource)

	allnodeapqns, err := apScanAPQNs(false)
	if err != nil {
		log.Printf("Plugin['%s']: failure trying to scan node APQNs: %s\n", p.resource, err)
		return fmt.Errorf("Plugin['%s']: fatal failure at start", p.resource)
	}

	p.apqns = p.filterAPQNs(p.ccset, allnodeapqns)
	log.Printf("Plugin['%s']: Found %d eligible APQNs: %s\n", p.resource, len(p.apqns), p.apqns)
	p.tellMetricsCollAboutAPQNs()

	p.devices = p.makePluginDevsFromAPQNs()
	log.Printf("Plugin['%s']: Derived %d plugin devices from the list of APQNs\n",
		p.resource, len(p.devices))
	p.tellMetricsCollAboutPluginDevs()

	p.stopChan = make(chan struct{})
	p.changedChan = make(chan struct{})

	go p.checkChangedLoop()

	return nil
}

func (p *ZCryptoResPlugin) Stop() error {

	log.Printf("Plugin['%s']: Stop()\n", p.resource)

	// clear apqns and plugin devices and tell metric collector about this
	p.apqns = nil
	p.tellMetricsCollAboutAPQNs()
	p.devices = nil
	p.tellMetricsCollAboutPluginDevs()

	// close the stop channel and thus trigger listener of this channel to stop their work
	close(p.stopChan)

	// wait until all users of the changed channel are done, then close the changed channel
	p.wgChChan.Wait()
	close(p.changedChan)

	return nil
}

func (p *ZCryptoResPlugin) GetDevicePluginOptions(context.Context, *kdp.Empty) (*kdp.DevicePluginOptions, error) {

	log.Printf("Plugin['%s']: GetDevicePluginOptions()\n", p.resource)

	return &kdp.DevicePluginOptions{PreStartRequired: false}, nil
}

func (p *ZCryptoResPlugin) ListAndWatch(e *kdp.Empty, s kdp.DevicePlugin_ListAndWatchServer) error {

	log.Printf("Plugin['%s']: ListAndWatch() Announcing %d devices: %s\n",
		p.resource, len(p.devices), p.devices)
	s.Send(&kdp.ListAndWatchResponse{Devices: p.devices})

	for {
		select {
		case <-p.stopChan:
			return nil
		case _, ok := <-p.changedChan:
			if !ok {
				return nil
			}
			log.Printf("Plugin['%s']: ListAndWatch() Re-announcing %d devices: %s\n",
				p.resource, len(p.devices), p.devices)
			s.Send(&kdp.ListAndWatchResponse{Devices: p.devices})
		}
	}
}

func (p *ZCryptoResPlugin) GetPreferredAllocation(ctx context.Context,
	req *kdp.PreferredAllocationRequest) (*kdp.PreferredAllocationResponse, error) {

	//log.Printf("Plugin['%s']: GetPreferredAllocation()\n", p.resource)

	return nil, nil
}

func (p *ZCryptoResPlugin) Allocate(ctx context.Context, req *kdp.AllocateRequest) (*kdp.AllocateResponse, error) {

	log.Printf("Plugin['%s']: Allocate(request=%v)\n", p.resource, req)

	rsp := new(kdp.AllocateResponse)
	for _, careq := range req.GetContainerRequests() {
		//fmt.Printf("debug Plugin['%s']: Allocate(): Container allocrequest=%v\n", p.resource, careq)
		carsp := kdp.ContainerAllocateResponse{}
		for _, id := range careq.GetDevicesIDs() {
			// parse device id
			//fmt.Printf("debug Plugin['%s']: Allocate(): Container request for device ID %v\n", p.resource, id)
			var card, queue, overcount int
			n, err := fmt.Sscanf(id, ApqnFmtStr, &card, &queue, &overcount)
			if err != nil || n < 3 {
				log.Printf("Plugin['%s']: Error parsing device id '%s'\n", p.resource, id)
				return nil, fmt.Errorf("Error parsing device id '%s'", id)
			}
			// check and maybe create a zcrypt device node
			znode := fmt.Sprintf("zcrypt-"+ApqnFmtStr, card, queue, overcount)
			if !zcryptNodeExists(znode) {
				log.Printf("Plugin['%s']: creating zcrypt device node '%s'\n", p.resource, znode)
				err = zcryptCreateSimpleNode(znode, card, queue)
				if err != nil {
					log.Printf("Plugin['%s']: Error creating zcrypt node '%s': %s\n", p.resource, znode, err)
					defer zcryptDestroyNode(znode)
					return nil, fmt.Errorf("Error creating zcrypt node '%s'", znode)
				}
			} else {
				//fmt.Printf("debug Plugin['%s']: zcrypt device node '%s' already exists\n", p.resource, znode)
			}
			// map the zcrypt device node to /dev/z90crypt inside the container
			dev := new(kdp.DeviceSpec)
			dev.HostPath = "/dev/" + znode
			dev.ContainerPath = "/dev/z90crypt"
			dev.Permissions = "rw"
			carsp.Devices = append(carsp.Devices, dev)
			// create AP bus and devices shadow sysfs for this container and mount them into the container
			apbusdir, apdevsdir, err := makeShadowApSysfs(id, p.ccset.Livesysfs, card, queue)
			if err != nil {
				log.Printf("Plugin['%s']: Error creating shadow sysfs for device '%s': %s\n", p.resource, id, err)
				defer zcryptDestroyNode(znode)
				return nil, fmt.Errorf("Error creating shadow sysfs for device '%s'", id)
			}
			carsp.Mounts = append(carsp.Mounts, &kdp.Mount{
				ContainerPath: "/sys/bus/ap",
				HostPath:      apbusdir,
				ReadOnly:      true})
			carsp.Mounts = append(carsp.Mounts, &kdp.Mount{
				ContainerPath: "/sys/devices/ap",
				HostPath:      apdevsdir,
				ReadOnly:      true})
			if p.ccset.Livesysfs > 0 {
				err = addLiveMounts(id, &carsp, card, queue)
				if err != nil {
					log.Printf("Plugin['%s']: Error adding live mounts for device '%s': %s\n", p.resource, id, err)
					defer zcryptDestroyNode(znode)
					return nil, fmt.Errorf("Error adding live mounts for device '%s'", id)
				}
			}
			p.tellMetricsCollAboutAlloc(id)
			// only one device per container supported
			break
		}
		rsp.ContainerResponses = append(rsp.ContainerResponses, &carsp)
	}

	log.Printf("Plugin['%s']: Allocate() response=%v\n", p.resource, rsp)

	return rsp, nil
}

func (p *ZCryptoResPlugin) PreStartContainer(context.Context, *kdp.PreStartContainerRequest) (*kdp.PreStartContainerResponse, error) {

	//log.Printf("Plugin['%s']: PreStartContainer()\n", p.resource)
	return nil, fmt.Errorf("PreStartContainer() not implemented")
}

func RunZCryptoResPlugins() {

	machineid, err := ccGetMachineId()
	if err != nil {
		log.Fatalf("Plugin: Fetching machine id failed: %s\n", err)
	}

	lister := &ZCryptoDPMLister{
		machineid: machineid,
	}

	mgr := dpm.NewManager(lister)
	mgr.Run()
}

func (p *ZCryptoResPlugin) tellMetricsCollAboutAPQNs() {
	MetricsCollAPQNs(p.resource, p.apqns)
}

func (p *ZCryptoResPlugin) tellMetricsCollAboutPluginDevs() {

	var devs []string

	for _, dev := range p.devices {
		if dev.Health == kdp.Healthy {
			devs = append(devs, dev.ID)
		}
	}
	MetricsCollPluginDevs(p.resource, devs)
}

func (p *ZCryptoResPlugin) tellMetricsCollAboutAlloc(zdevnode string) {
	MetricsCollNotifyAboutAlloc(p.resource, zdevnode)
}
