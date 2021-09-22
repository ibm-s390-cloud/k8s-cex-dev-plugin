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
	"time"

	"github.com/kubevirt/device-plugin-manager/pkg/dpm"
	kdp "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	baseResourceName    = "cex.s390.ibm.com" // base resource name
)

var (
	apqnOverCommitLimit = getenvint("APQN_OVERCOMMIT_LIMIT", 1, 1) // overcommit limit: 1 is no overcommit
	apqnsCheckInterval  = time.Duration(getenvint("APQN_CHECK_INTERVAL", 30, 10)) // device health check interval in seconds
)

type ZCryptoDPMLister struct {
	machineid    string
}

type ZCryptoResPlugin struct {
	resource    string
	lister      *ZCryptoDPMLister
	ccset       *CryptoConfigSet
	tag         []byte
	apqns       APQNList
	devices     []*kdp.Device
	changedChan chan struct{}
	stopChan    chan struct{}
}

func (l *ZCryptoDPMLister) GetResourceNamespace() string {

	log.Printf("Plugin: Announcing '%s' as our resource namespace\n", baseResourceName)

	return baseResourceName
}

func (z *ZCryptoDPMLister) Discover(nameslist chan dpm.PluginNameList) {

	listofsetnames := GetCurrentCryptoConfig().GetListOfSetNames()

	log.Printf("Plugin: Register plugins for these CryptoConfigSets: %v\n", listofsetnames)

	nameslist <- dpm.PluginNameList(listofsetnames)
}

func (z *ZCryptoDPMLister) NewPlugin(resource string) dpm.PluginInterface {

	log.Printf("Plugin: NewPlugin('%s')\n", resource)

	ccset, tag := GetCurrentCryptoConfigSet(nil, resource, nil)

	p := &ZCryptoResPlugin{
		lister:   z,
		resource: resource,
		ccset:    ccset,
		tag:      tag,
	}
	return p
}

func (p *ZCryptoResPlugin) filterAPQNs(apqnlist APQNList) APQNList {

	var apqns APQNList

	ccset, tag := GetCurrentCryptoConfigSet(p.ccset, p.resource, p.tag)
	p.ccset, p.tag = ccset, tag

	if ccset == nil {
		return apqns
	}

	for _, a := range apqnlist {
		for _, c := range p.ccset.APQNDefs {
			if a.Adapter != c.Adapter || a.Domain != c.Domain {
				continue
			}
			if len(c.MachineId) > 0 && p.lister.machineid != c.MachineId {
				continue
			}
			apqns = append(apqns, a)
		}
	}

	return apqns
}

func (p *ZCryptoResPlugin) makePluginDevsFromAPQNs() []*kdp.Device {

	var devices []*kdp.Device

	for _, a := range p.apqns {
		health := kdp.Healthy
		if !a.Online {
			health = kdp.Unhealthy
		}
		for i := 0; i < apqnOverCommitLimit; i++ {
			devices = append(devices, &kdp.Device{
				ID:     fmt.Sprintf("apqn-%d-%d-%d", a.Adapter, a.Domain, i),
				Health: health,
			})
		}
	}

	return devices
}

func (p *ZCryptoResPlugin) checkApqnsChanged() bool {

	log.Printf("Plugin['%s']: checkApqnsChanged() rescanning available APQNs\n", p.resource)

	allnodeapqns, err := apScanAPQNs(false)
	if err != nil {
		log.Printf("Plugin['%s']: failure trying to rescan node APQNs: %s\n", p.resource, err)
		return false
	}

	apqns := p.filterAPQNs(allnodeapqns)
	if apEqualAPQNLists(apqns, p.apqns) {
		log.Printf("Plugin['%s']: Rescan found %d eligible APQNs but no changes\n", p.resource, len(apqns))
		return false
	} else {
		log.Printf("Plugin['%s']: Rescan found %d eligible APQNs (with changes): %s\n", p.resource, len(apqns), apqns)
		p.apqns = apqns
		p.devices = p.makePluginDevsFromAPQNs()
		log.Printf("Plugin['%s']: Derived %d plugin devices from the list of APQNs\n",
			p.resource, len(p.devices))
		return true
	}
}

func (p *ZCryptoResPlugin) checkApqnsChangedLoop() {

	tick := time.NewTicker(apqnsCheckInterval * time.Second)

ForLoop:
	for {
		select {
		case <-p.stopChan:
			tick.Stop()
			break ForLoop
		case <-tick.C:
			if p.checkApqnsChanged() {
				p.changedChan <- struct{}{}
			}
		}
	}
}

func (p *ZCryptoResPlugin) Start() error {

	log.Printf("Plugin['%s']: Start()\n", p.resource)

	allnodeapqns, err := apScanAPQNs(false)
	if err != nil {
		log.Printf("Plugin['%s']: failure trying to scan node APQNs: %s\n", p.resource, err)
		return fmt.Errorf("Plugin['%s']: fatal failure at start\n", p.resource)
	}

	p.apqns = p.filterAPQNs(allnodeapqns)
	log.Printf("Plugin['%s']: Found %d eligible APQNs: %s\n", p.resource, len(p.apqns), p.apqns)

	p.devices = p.makePluginDevsFromAPQNs()
	log.Printf("Plugin['%s']: Derived %d plugin devices from the list of APQNs\n",
		p.resource, len(p.devices))

	p.stopChan = make(chan struct{})
	p.changedChan = make(chan struct{})

	go p.checkApqnsChangedLoop()

	return nil
}

func (p *ZCryptoResPlugin) Stop() error {

	log.Printf("Plugin['%s']: Stop()\n", p.resource)

	close(p.stopChan)
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
		case <-p.changedChan:
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
			n, err := fmt.Sscanf(id, "apqn-%d-%d-%d", &card, &queue, &overcount)
			if err != nil || n < 3 {
				log.Printf("Plugin['%s']: Error parsing device id '%s'\n", p.resource, id)
				return nil, fmt.Errorf("Error parsing device id '%s'\n", id)
			}
			// check and maybe create a zcrypt device node
			znode := fmt.Sprintf("zcrypt-apqn-%d-%d-%d", card, queue, overcount)
			if !zcryptNodeExists(znode) {
				log.Printf("Plugin['%s']: creating zcrypt device node '%s'\n", p.resource, znode)
				err = zcryptCreateSimpleNode(znode, card, queue)
				if err != nil {
					log.Printf("Plugin['%s']: Error creating zcrypt node '%s': %s\n", p.resource, znode, err)
					defer zcryptDestroyNode(znode)
					return nil, fmt.Errorf("Error creating zcrypt node '%s'\n", znode)
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
			apbusdir, apdevsdir, err := makeShadowApSysfs(id, card, queue)
			if err != nil {
				log.Printf("Plugin['%s']: Error creating shadow sysfs for device '%s': %s\n", p.resource, id, err)
				defer zcryptDestroyNode(znode)
				return nil, fmt.Errorf("Error creating shadow sysfs for device '%s'\n", id)
			}
			carsp.Mounts = append(carsp.Mounts, &kdp.Mount{
				ContainerPath: "/sys/bus/ap",
				HostPath:      apbusdir,
				ReadOnly:      true})
			carsp.Mounts = append(carsp.Mounts, &kdp.Mount{
				ContainerPath: "/sys/devices/ap",
				HostPath:      apdevsdir,
				ReadOnly:      true})
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
	return nil, fmt.Errorf("PreStartContainer() not implemented\n")
}

func RunZCryptoResPlugins() {

	machineid, err := ccGetMachineId()
	if err != nil {
		log.Fatalf("Plugin: Fetching machine id failed: %s\n", err)
	}

	lister := &ZCryptoDPMLister{
		machineid:    machineid,
	}

	mgr := dpm.NewManager(lister)
	mgr.Run()
}
