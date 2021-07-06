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
 * zcrypt multiple device nodes handling functions
 */

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	zcryptclassdir     = "/sys/class/zcrypt"
	zcryptvdevdir      = "/sys/devices/virtual/zcrypt"
	zcryptnodefilemode = 0666
)

func zcryptHasNodesSupport() bool {

	_, err := os.Stat(zcryptclassdir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Zcrypt: No zcrypt multiple nodes support ('%s' does not exist)\n", zcryptclassdir)
			return false
		} else {
			log.Printf("Zcrypt: Error reading zcrypt multiple nodes support dir: %s\n", err)
			return false
		}
	}

	return true
}

func zcryptNodeExists(nodename string) bool {

	dirname := zcryptvdevdir + "/" + nodename
	_, err := os.Stat(dirname)
	if err != nil {
		return false
	}

	return true
}

func zcryptDestroyNode(nodename string) error {

	// destroy the zcrypt device node via writing to /sys/class/zcrypt/destroy
	destroyfname := zcryptclassdir + "/" + "destroy"
	f, err := os.OpenFile(destroyfname, os.O_WRONLY, 0)
	if err != nil {
		log.Printf("Zcrypt: Can't open file '%s': %s\n", destroyfname, err)
		return err
	}
	defer f.Close()
	_, err = f.WriteString(nodename)
	if err != nil {
		log.Printf("Zcrypt: Error writing to '%s': %s\n", destroyfname, err)
		return err
	}

	return nil
}

func zcryptCreateNode(nodename string) error {

	// create the new zcrpyt device node via writing to /sys/class/zcrypt/create
	createfname := zcryptclassdir + "/" + "create"
	f, err := os.OpenFile(createfname, os.O_WRONLY, 0)
	if err != nil {
		log.Printf("Zcrypt: Can't open file '%s': %s\n", createfname, err)
		return err
	}
	_, err = f.WriteString(nodename)
	if err != nil {
		log.Printf("Zcrypt: Error writing to '%s': %s\n", createfname, err)
		f.Close()
		return err
	}
	f.Close()

	// wait until the device node file in /dev is created via udev
	devname := "/dev/" + nodename
	ok := false
	for w := 25; !ok && w <= 3200; w *= 2 {
		_, err := os.Stat(devname)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Printf("Zcrypt: Error waiting for device node '%s' to appear: %s\n", devname, err)
				zcryptDestroyNode(nodename)
				return fmt.Errorf("Zcrypt: Error waiting for device node '%s' to appear: %w\n", devname, err)
			}
			// powernap
			time.Sleep(time.Duration(w) * time.Millisecond)
		} else {
			// the new device node exists
			ok = true
		}
	}
	if !ok {
		log.Printf("Zcrypt: Timeout waiting for device node '%s' to appear\n", devname)
		zcryptDestroyNode(nodename)
		return fmt.Errorf("Zcrypt: Timeout waiting for device node '%s' to appear", devname)
	}

	// adjust filemode for this new zcrypt device node
	err = os.Chmod(devname, zcryptnodefilemode)
	if err != nil {
		log.Printf("Zcrypt: Error changing the filemode for the device node: %s\n", err)
		zcryptDestroyNode(nodename)
		return fmt.Errorf("Zcrypt: Error changing the filemode for the device node: %w\n", err)
	}

	log.Printf("Zcrypt: Successfully created new zcrypt device node '%s'\n", nodename)

	return nil
}

func zcryptAddAdaptersToNode(nodename string, adapters ...int) error {

	// add adapter by writing +<adapter> to /sys/devices/virtual/zcrypt/<nodename>/apmask
	apmaskfname := zcryptvdevdir + "/" + nodename + "/" + "apmask"
	f, err := os.OpenFile(apmaskfname, os.O_WRONLY, 0)
	if err != nil {
		log.Printf("Zcrypt: Can't open file '%s': %s\n", apmaskfname, err)
		return err
	}
	defer f.Close()

	str := ""
	for _, ap := range adapters {
		if len(str) > 0 {
			str = str + ","
		}
		str = str + fmt.Sprintf("+%d", ap)
	}
	if len(str) > 0 {
		str = str + "\n"
		_, err = f.WriteString(str)
		if err != nil {
			log.Printf("Zcrypt: Error writing to '%s': %s\n", apmaskfname, err)
			return err
		}
	}

	return nil
}

func zcryptAddDomainsToNode(nodename string, domains ...int) error {

	// add domains by writing +<domain> to /sys/devices/virtual/zcrypt/<nodename>/aqmask
	aqmaskfname := zcryptvdevdir + "/" + nodename + "/" + "aqmask"
	f, err := os.OpenFile(aqmaskfname, os.O_WRONLY, 0)
	if err != nil {
		log.Printf("Zcrypt: Can't open file '%s': %s\n", aqmaskfname, err)
		return err
	}
	defer f.Close()

	var b strings.Builder
	// 3 for max size, 1 for +, 1 for comma
	b.Grow(len(domains) * (3 + 1 + 1))

	for i, dom := range domains {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "+%d", dom)
	}
	if len(domains) > 0 {
		_, err = fmt.Fprintln(f, b.String())
		if err != nil {
			log.Printf("Zcrypt: Error writing to '%s': %s\n", aqmaskfname, err)
			return err
		}
	}

	return nil
}

func zcryptAddIoctlsToNode(nodename string, ioctls ...int) error {

	// add ioctls by writing +<ioctl> to /sys/devices/virtual/zcrypt/<nodename>/ioctlmask
	ioctlmaskfname := zcryptvdevdir + "/" + nodename + "/" + "ioctlmask"
	f, err := os.OpenFile(ioctlmaskfname, os.O_WRONLY, 0)
	if err != nil {
		log.Printf("Zcrypt: Can't open file '%s': %s\n", ioctlmaskfname, err)
		return err
	}
	defer f.Close()

	if len(ioctls) == 0 {
		// no ioctls given means all
		ioctls = make([]int, 0, 256)
		for i := 0; i < 256; i++ {
			ioctls = append(ioctls, i)
		}
	}

	var b strings.Builder
	// 3 for max len, 1 for +, 1 for comma
	b.Grow(len(ioctls) * (3 + 1 + 1))
	for i, ioctl := range ioctls {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "+%d", ioctl)
	}
	_, err = fmt.Fprintln(f, b.String())
	if err != nil {
		log.Printf("Zcrypt: Error writing to '%s': %s\n", ioctlmaskfname, err)
		return fmt.Errorf("Zcrypt: Error writing to '%s': %w\n", ioctlmaskfname, err)
	}

	return nil
}

func zcryptCreateSimpleNode(nodename string, adapter, domain int) error {

	if err := zcryptCreateNode(nodename); err != nil {
		return fmt.Errorf("Zcrypt: zcryptCreateNode('%s') failed: %w\n", nodename, err)
	}

	if err := zcryptAddAdaptersToNode(nodename, adapter); err != nil {
		return fmt.Errorf("Zcrypt: zcryptAddAdaptersToNode('%s') failed: %w\n", nodename, err)
	}

	if err := zcryptAddDomainsToNode(nodename, domain); err != nil {
		return fmt.Errorf("Zcrypt: zcryptAddDomainsToNode('%s') failed: %w\n", nodename, err)
	}

	if err := zcryptAddIoctlsToNode(nodename); err != nil {
		return fmt.Errorf("Zcrypt: zcryptAddIoctlsToNode('%s') failed: %w\n", nodename, err)
	}

	log.Printf("Zcrypt: simple node '%s' for APQN(%d,%d) created\n", nodename, adapter, domain)

	return nil
}

func zcryptFetchActiveNodes() ([]string, error) {

	var nodes []string

	_, err := os.Stat(zcryptvdevdir)
	if err != nil && os.IsNotExist(err) {
		return nodes, nil
	}

	files, err := ioutil.ReadDir(zcryptvdevdir)
	if err != nil {
		log.Printf("Zcrypt: Can't read directory %s: %s\n", zcryptvdevdir, err)
		return nil, fmt.Errorf("Zcrypt: Can't read directory %s: %s\n", zcryptvdevdir, err)
	}

	for _, f := range files {
		match, _ := regexp.MatchString("zcrypt-apqn-.*", f.Name())
		if match {
			nodes = append(nodes, f.Name())
		}
	}

	return nodes, nil
}
