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
 * kubernetes device plugin implementation
 */

package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"strings"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	k8spapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	zcryptDevSocketName   = "zcrypt-%s-plugin.sock"
	zcryptResourceName    = "ibm.com/cex-%s"
	zcryptGRpcTimeout     = 10 // gRPC timeout in seconds
	zcryptHealthCheck     = 30 // devices health check interval in seconds
	zcryptAPQNOverCommits = 1  // 1 no overcommit, > 1 overcommit limit (untested)
)

func checkK8DevicePluginPath() bool {

	var k8devplugindir string = k8spapi.DevicePluginPath
	_, err := os.Stat(k8devplugindir)
	if err != nil && os.IsNotExist(err) {
		log.Printf("plugin: k8 dev plugin dir '%s' disappeared", k8devplugindir)
		return false
	}

	return true
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

// ZcryptDevPlugin

type ZcryptDevPlugin struct {
	cextype      string
	resourceName string
	socketName   string
	server       *grpc.Server
	stopChan     chan struct{}
	healthChan   chan bool
	apqns        APQNList
	devices      []*k8spapi.Device
}

func scanForEligibleAPQNs(cextype string) APQNList {

	allAPQNs, err := apScanAPQNs()
	if err != nil {
		log.Printf("plugin: apScanAPQNs() returned with failure, could not find any eligible APQNs\n")
		return nil
	}

	return allAPQNs.filterMode(cextype)
}

func makePluginDevsFromAPQNs(apqns APQNList) []*k8spapi.Device {

	var devices []*k8spapi.Device

	for _, a := range apqns {
		health := k8spapi.Healthy
		if !a.online {
			health = k8spapi.Unhealthy
		}
		if zcryptAPQNOverCommits > 1 {
			for i := 0; i < zcryptAPQNOverCommits; i++ {
				devices = append(devices, &k8spapi.Device{
					ID:     fmt.Sprintf("apqn_%d_%d-%d", a.adapter, a.domain, i),
					Health: health,
				})
			}
		} else {
			devices = append(devices, &k8spapi.Device{
				ID:     fmt.Sprintf("apqn_%d_%d", a.adapter, a.domain),
				Health: health,
			})
		}
	}

	return devices
}

func NewZcryptDevPlugin(cextype string) (*ZcryptDevPlugin, error) {

	switch cextype {
	case "ep11", "cca", "accel":
		break
	default:
		log.Printf("plugin: Unknown/unsupported cextype '%s'\n", cextype)
		return nil, fmt.Errorf("Unknown/unsupported cextype '%s'", cextype)
	}

	apqns := scanForEligibleAPQNs(cextype)
	if len(apqns) < 1 {
		log.Printf("plugin: No eligible APQNs for cextype '%s' found\n", cextype)
	} else {
		log.Printf("plugin: Found %d eligible APQNs for cextype '%s':\n", len(apqns), cextype)
		log.Printf("plugin: %s\n", apqns)
	}

	return &ZcryptDevPlugin{
		cextype:      cextype,
		resourceName: fmt.Sprintf(zcryptResourceName, cextype),
		socketName:   k8spapi.DevicePluginPath + fmt.Sprintf(zcryptDevSocketName, cextype),
		stopChan:     make(chan struct{}),
		healthChan:   make(chan bool),
		apqns:        apqns,
		devices:      makePluginDevsFromAPQNs(apqns),
	}, nil
}

func (z *ZcryptDevPlugin) String() string {

	var b strings.Builder
	b.Grow(len(z.devices) * (10 + 3 + 15 + 9))

	for i, v := range z.devices {
		if i > 0 {
			b.WriteString(" ")
		}
		fmt.Fprintf(&b, "dev[%d]={%v, %v}", i, v.ID, v.Health)
	}
	return b.String()
}

func (z *ZcryptDevPlugin) cleanup() {

	if z.server != nil {
		z.server.Stop()
		z.server = nil
		close(z.stopChan)
	}

	if err := os.Remove(z.socketName); err != nil && !os.IsNotExist(err) {
		log.Printf("plugin: Remove of grpc socket %s failed: %s\n", z.socketName, err)
	}
}

func (z *ZcryptDevPlugin) doHealthCheck() bool {

	hasupdates := false
	apqns := scanForEligibleAPQNs(z.cextype)

	if len(apqns) != len(z.apqns) {
		log.Printf("plugin: Old number of eligible APQNs = %d, new number = %d", len(z.apqns), len(apqns))
		hasupdates = true
	}

	for _, n := range apqns {
		if hasupdates {
			break
		}
		for _, o := range z.apqns {
			if hasupdates {
				break
			}
			if n.adapter == o.adapter && n.domain == o.domain {
				if n.gen != o.gen {
					log.Printf("plugin: APQN(%d,%d) gen changed from %v to %v\n",
						n.adapter, n.domain, o.gen, n.gen)
					hasupdates = true
				}
				if n.mode != o.mode {
					log.Printf("plugin: APQN(%d,%d) mode changed from %v to %v\n",
						n.adapter, n.domain, o.mode, n.mode)
					hasupdates = true
				}
				if n.online != o.online {
					log.Printf("plugin: APQN(%d,%d) online state changed from %v to %v\n",
						n.adapter, n.domain, o.online, n.online)
					hasupdates = true
				}
			}
		}
	}

	if hasupdates {
		z.apqns = apqns
		z.devices = makePluginDevsFromAPQNs(apqns)
	}

	return hasupdates
}

func (z *ZcryptDevPlugin) healthCheckLoop() {

	tick := time.NewTicker(zcryptHealthCheck * time.Second)

ForLoop:
	for {
		select {
		case <-z.stopChan:
			tick.Stop()
			break ForLoop
		case <-tick.C:
			z.healthChan <- z.doHealthCheck()
		}
	}
}

func (z *ZcryptDevPlugin) Start() error {

	log.Println("plugin: starting...")

	z.cleanup()

	socket, err := net.Listen("unix", z.socketName)
	if err != nil {
		log.Printf("plugin: Creation of unix domain socket failed: %s\n", err)
		return err
	}
	z.server = grpc.NewServer([]grpc.ServerOption{}...)
	k8spapi.RegisterDevicePluginServer(z.server, z)

	go z.server.Serve(socket)

	con, err := dial(z.socketName, zcryptGRpcTimeout*time.Second)
	if err != nil {
		log.Printf("plugin: Connection test to own unix domain socket failed: %s\n", err)
		return err
	}
	con.Close()

	go z.healthCheckLoop()

	err = z.Register(z.resourceName)
	if err != nil {
		log.Printf("plugin: Registration for resource %s failed: %s\n", z.resourceName, err)
		return err
	}
	log.Printf("plugin: Registration for resource %s successful\n", z.resourceName)

	log.Println("plugin: start successful")

	return nil
}

func (z *ZcryptDevPlugin) Stop() error {

	log.Println("plugin: stopping...")

	z.cleanup()

	log.Println("plugin: stopped")
	return nil
}

func (z *ZcryptDevPlugin) Register(resourceName string) error {

	log.Println("plugin: register...")

	con, err := dial(k8spapi.KubeletSocket, zcryptGRpcTimeout*time.Second)
	if err != nil {
		log.Printf("plugin: Connection to kublet failed: %s\n", err)
		return err
	}
	defer con.Close()

	kubclient := k8spapi.NewRegistrationClient(con)

	regrequest := &k8spapi.RegisterRequest{
		Version:      k8spapi.Version,
		Endpoint:     path.Base(z.socketName),
		ResourceName: resourceName,
	}

	if _, err = kubclient.Register(context.Background(), regrequest); err != nil {
		log.Printf("plugin: Register with kublet failed: %s\n", err)
		return err
	}

	log.Println("plugin: register successful")

	return nil
}

// ZcryptDevPlugin DevicePluginServer interface

func (z *ZcryptDevPlugin) GetDevicePluginOptions(context.Context, *k8spapi.Empty) (*k8spapi.DevicePluginOptions, error) {

	return &k8spapi.DevicePluginOptions{PreStartRequired: false}, nil
}

func (z *ZcryptDevPlugin) ListAndWatch(e *k8spapi.Empty, s k8spapi.DevicePlugin_ListAndWatchServer) error {

	log.Printf("plugin: Announcing devices: %s\n", z)
	s.Send(&k8spapi.ListAndWatchResponse{Devices: z.devices})

	for {
		select {
		case <-z.stopChan:
			return nil
		case changed := <-z.healthChan:
			if changed {
				log.Printf("plugin: Re-announcing devices: %s\n", z)
				s.Send(&k8spapi.ListAndWatchResponse{Devices: z.devices})
			}
		}
	}
}

func (z *ZcryptDevPlugin) Allocate(ctx context.Context, req *k8spapi.AllocateRequest) (*k8spapi.AllocateResponse, error) {

	log.Printf("plugin: Allocate() called, request=%v\n", req)

	rsp := new(k8spapi.AllocateResponse)
	for _, careq := range req.GetContainerRequests() {
		//fmt.Printf("debug: Allocate(): Container allocrequest=%v\n", careq)
		carsp := k8spapi.ContainerAllocateResponse{}
		for _, id := range careq.GetDevicesIDs() {
			//fmt.Printf("debug: Allocate(): Container request for device ID %v\n", id)
			var card, queue int
			n, err := fmt.Sscanf(id, "apqn_%d_%d", &card, &queue)
			if err != nil || n < 2 {
				log.Printf("plugin: Error parsing device id '%s'\n", id)
				return nil, errors.New("Error parsing device id")
			}
			znode := fmt.Sprintf("zcrypt_apqn_%d_%d", card, queue)
			if !zcryptNodeExists(znode) {
				//fmt.Printf("debug: creating zcrypt device node '%s'\n", znode)
				err = zcryptCreateSimpleNode(znode, card, queue)
				if err != nil {
					log.Printf("plugin: Error creating zcrypt node: %s\n", err)
					defer zcryptDestroyNode(znode)
					return nil, errors.New("Error creating zcrypt node")
				}
			} else {
				//fmt.Printf("debug: zcrypt device node '%s' already exists\n", znode)
			}
			dev := new(k8spapi.DeviceSpec)
			dev.HostPath = "/dev/" + znode
			dev.ContainerPath = "/dev/z90crypt"
			dev.Permissions = "rw"
			carsp.Devices = append(carsp.Devices, dev)
			// only one device per container
			break
		}
		rsp.ContainerResponses = append(rsp.ContainerResponses, &carsp)
	}

	log.Printf("plugin: Allocate() response=%v\n", rsp)

	return rsp, nil
}

func (z *ZcryptDevPlugin) PreStartContainer(context.Context, *k8spapi.PreStartContainerRequest) (*k8spapi.PreStartContainerResponse, error) {

	//fmt.Println("debug: PreStartContainer()")

	return nil, errors.New("PreStartContainer() not implemented")
}
