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
 * shadow ap sysfs functions
 */

package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"syscall"

	kdp "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

var apbusdir = getenvstr("APSYSFS_BUSDIR", "/sys/bus/ap")
var apdevsdir = getenvstr("APSYSFS_DEVSDIR", "/sys/devices/ap")
var shadowbasedir = getenvstr("SHADOWSYSFS_BASEDIR", "/var/tmp/shadowsysfs")

// sys/bus/ap
var sys_bus_ap_copyfiles = []string{
	"ap_interrupts",
	"ap_max_domain_id",
	"poll_thread",
	"poll_timeout",
}
var sys_bus_ap_maybecopyfiles = []string{
	"ap_max_adapter_id",
}

// sys/devices/ap/card<xx>/
var sys_devices_ap_card_copyfiles = []string{
	"ap_functions",
	"depth",
	"hwtype",
	"raw_hwtype",
	"type",
}
var sys_devices_ap_card_maybecopyfiles = []string{
	"API_ordinalnr",
	"FW_version",
	"op_modes",
	"serialnr",
}
var sys_devices_ap_card_fileswithvalue = []struct{ name, value string }{
	{"load", "0\n"},
	{"online", "1\n"},
	{"pendingq_count", "0\n"},
	{"request_count", "0\n"},
	{"requestq_count", "0\n"},
}
var sys_devices_ap_card_fileswithvalue_live = []struct{ name, value string }{
	{"load", "0\n"},
}
var sys_devices_ap_card_links_to_queuedir = []struct{ name, value string }{
	{"online", "1\n"},
	{"pendingq_count", "0\n"},
	{"request_count", "0\n"},
	{"requestq_count", "0\n"},
}

// sys/devices/ap/card<xx>/<xx>.<yyyy>/
var sys_devices_ap_queue_copyfiles = []string{
	"interrupt",
	"reset",
}
var sys_devices_ap_queue_maybecopyfiles = []string{
	"mkvps",
	"op_modes",
}
var sys_devices_ap_queue_fileswithvalue = []struct{ name, value string }{
	{"load", "0\n"},
	{"online", "1\n"},
	{"pendingq_count", "0\n"},
	{"request_count", "0\n"},
	{"requestq_count", "0\n"},
}

func shadowSysfsInit() bool {

	info, err := os.Stat(shadowbasedir)
	if os.IsNotExist(err) {
		err := os.MkdirAll(shadowbasedir, 0755)
		if err != nil {
			log.Printf("Shadowsysfs: Failure on creating base directory %s: %v\n", shadowbasedir, err)
			return false
		}
		log.Printf("Shadowsysfs: Base directory %s created\n", shadowbasedir)
		return true
	} else if err != nil {
		log.Printf("Shadowsysfs: Invalid base dir %s: %v\n", shadowbasedir, err)
		return false
	}
	if !info.IsDir() || (info.Mode()&0700 != 0700) {
		log.Printf("Shadowsysfs: Invalid base dir %s: no directory or invalid permissions\n", shadowbasedir)
		return false
	}

	return true
}

func makeShadowApSysfs(id string, livesysfs, adapter, domain int) (string, string, error) {

	exists := func(name string) bool {
		_, err := os.Stat(name)
		if err == nil {
			return true
		}
		return false
	}
	makedir := func(dirname string) error {
		//fmt.Printf("debug: makedir(%s)\n", dirname)
		err := os.MkdirAll(dirname, 0755)
		if err != nil {
			log.Printf("Shadowsysfs: failed to create shadow sysfs dir %s: %s\n", dirname, err)
			return fmt.Errorf("Shadowsysfs: Failed to create shadow sysfs dir %s: %s", dirname, err)
		}
		return nil
	}
	makefile := func(filename, content string) error {
		//fmt.Printf("debug: makefile(%s)\n", filename)
		rawdata := []byte(content)
		err := os.WriteFile(filename, rawdata, 0444)
		if err != nil {
			log.Printf("Shadowsysfs: failed to write shadow sysfs file %s: %s\n", filename, err)
			return fmt.Errorf("Shadowsysfs: Failed to write shadow sysfs file %s: %s", filename, err)
		}
		return nil
	}
	make256bitmaskfile := func(filename string, fill byte, mods ...int) error {
		//fmt.Printf("debug: make256bitmaskffile(%s)\n", filename)
		var mask [32]byte
		for i := 0; i < len(mask); i++ {
			mask[i] = fill
		}
		for _, m := range mods {
			var b byte = 0x80 >> (m % 8)
			if fill == 0 {
				mask[m/8] |= b
			} else {
				mask[m/8] &= ^b
			}
		}
		var b strings.Builder
		b.Grow(2 + 32*2 + 1)
		b.WriteString("0x")
		for i := 0; i < len(mask); i++ {
			fmt.Fprintf(&b, "%02x", mask[i])
		}
		b.WriteString("\n")
		return makefile(filename, b.String())
	}
	copyfile := func(src, dst string) error {
		//fmt.Printf("debug: copyfile(%s,%s)\n", src, dst)
		rawdata, err := os.ReadFile(src)
		if err != nil {
			log.Printf("Shadowsysfs: failed to read sysfs file %s: %s\n", src, err)
			return fmt.Errorf("Shadowsysfs: Failed to read sysfs file %s: %s", src, err)
		}
		err = os.WriteFile(dst, rawdata, 0444)
		if err != nil {
			log.Printf("Shadowsysfs: failed to write shadow sysfs file %s: %s\n", dst, err)
			return fmt.Errorf("Shadowsysfs: Failed to write shadow sysfs file %s: %s", dst, err)
		}
		return nil
	}
	copyfiles := func(srcdir, dstdir string, files []string) error {
		for _, f := range files {
			src := fmt.Sprintf("%s/%s", srcdir, f)
			dst := fmt.Sprintf("%s/%s", dstdir, f)
			err := copyfile(src, dst)
			if err != nil {
				return err
			}
		}
		return nil
	}
	maybecopyfiles := func(srcdir, dstdir string, files []string) error {
		for _, f := range files {
			src := fmt.Sprintf("%s/%s", srcdir, f)
			dst := fmt.Sprintf("%s/%s", dstdir, f)
			if exists(src) {
				err := copyfile(src, dst)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	makelink := func(src, dst string) error {
		//fmt.Printf("debug: makelink(%s,%s)\n", src, dst)
		err := os.Symlink(dst, src)
		if err != nil {
			log.Printf("Shadowsysfs: failed to create symlink %s -> %s: %s\n", src, dst, err)
			return fmt.Errorf("Shadowsysfs: Failed to create symlink %s -> %s: %s", src, dst, err)
		}
		return nil
	}

	var err error
	var shadowdir, shadowapbusdir, shadowapdevsdir string

	carddir := fmt.Sprintf("card%02x", adapter)
	queuedir := fmt.Sprintf("%02x.%04x", adapter, domain)

	// set umask to 0
	oldumask := syscall.Umask(0000)
	defer syscall.Umask(oldumask)

	// create shadow base dir if it does not exist (should be handled by initContainer)
	_, err = os.Stat(shadowbasedir)
	if err != nil {
		log.Printf("Shadowsysfs: missing shadow base dir %s: %s\n", shadowbasedir, err)
		return "", "", fmt.Errorf("Shadowsysfs: missing shadow base dir %s: %s", shadowbasedir, err)
	}

	// create shadow dir for this id
	shadowdir = fmt.Sprintf("%s/sysfs-%s", shadowbasedir, id)
	os.RemoveAll(shadowdir)
	err = makedir(shadowdir)
	if err != nil {
		return "", "", err
	}

	for {
		// shadow sys/bus/ap
		shadowapbusdir = fmt.Sprintf("%s/bus/ap", shadowdir)
		if err = makedir(shadowapbusdir); err != nil {
			break
		}
		if err = copyfiles(apbusdir, shadowapbusdir, sys_bus_ap_copyfiles); err != nil {
			break
		}
		if err = maybecopyfiles(apbusdir, shadowapbusdir, sys_bus_ap_maybecopyfiles); err != nil {
			break
		}
		if err = make256bitmaskfile(shadowapbusdir+"/ap_adapter_mask", 0x00, adapter); err != nil {
			break
		}
		if err = make256bitmaskfile(shadowapbusdir+"/ap_control_domain_mask", 0x00); err != nil {
			break
		}
		if err = makefile(shadowapbusdir+"/ap_domain", fmt.Sprintf("%d\n", domain)); err != nil {
			break
		}
		if err = make256bitmaskfile(shadowapbusdir+"/apmask", 0xff); err != nil {
			break
		}
		if err = make256bitmaskfile(shadowapbusdir+"/ap_usage_domain_mask", 0x00, domain); err != nil {
			break
		}
		if err = make256bitmaskfile(shadowapbusdir+"/aqmask", 0xff); err != nil {
			break
		}

		// shadow sys/device/ap
		shadowapdevsdir = fmt.Sprintf("%s/devices/ap", shadowdir)
		err = makedir(shadowapdevsdir)
		if err != nil {
			break
		}
		// shadow sys/devices/ap/card<xx>
		apcarddir := fmt.Sprintf("%s/%s", apdevsdir, carddir)
		shadowcarddir := fmt.Sprintf("%s/%s", shadowapdevsdir, carddir)
		err = makedir(shadowcarddir)
		if err != nil {
			break
		}
		err = copyfiles(apcarddir, shadowcarddir, sys_devices_ap_card_copyfiles)
		if err != nil {
			break
		}
		err = maybecopyfiles(apcarddir, shadowcarddir, sys_devices_ap_card_maybecopyfiles)
		if err != nil {
			break
		}
		if livesysfs > 0 {
			log.Printf("Shadowsysfs: creating live sysfs\n")
			for _, e := range sys_devices_ap_card_fileswithvalue_live {
				if err = makefile(shadowcarddir+"/"+e.name, e.value); err != nil {
					break
				}
			}
			for _, e := range sys_devices_ap_card_links_to_queuedir {
				linksrc := fmt.Sprintf("%s/%s", shadowcarddir, e.name)
				linkdst := fmt.Sprintf("%02x.%04x/%s", adapter, domain, e.name)
				if err = makelink(linksrc, linkdst); err != nil {
					break
				}
			}
		} else {
			log.Printf("Shadowsysfs: creating static sysfs\n")
			for _, e := range sys_devices_ap_card_fileswithvalue {
				if err = makefile(shadowcarddir+"/"+e.name, e.value); err != nil {
					break
				}
			}
		}
		if err != nil {
			break
		}
		if err = makelink(shadowcarddir+"/driver", "../../../bus/ap/drivers/cex4card"); err != nil {
			break
		}
		if err = makelink(shadowcarddir+"/subsystem", "../../../bus/ap"); err != nil {
			break
		}

		// shadow sys/devices/ap/card<xx>/<xx>.<yyyy>
		apqueuedir := fmt.Sprintf("%s/%s", apcarddir, queuedir)
		shadowqueuedir := fmt.Sprintf("%s/%s", shadowcarddir, queuedir)
		if err = makedir(shadowqueuedir); err != nil {
			break
		}
		if err = copyfiles(apqueuedir, shadowqueuedir, sys_devices_ap_queue_copyfiles); err != nil {
			break
		}
		if err = maybecopyfiles(apqueuedir, shadowqueuedir, sys_devices_ap_queue_maybecopyfiles); err != nil {
			break
		}
		for _, e := range sys_devices_ap_queue_fileswithvalue {
			if err = makefile(shadowqueuedir+"/"+e.name, e.value); err != nil {
				break
			}
		}
		if err != nil {
			break
		}
		if err = makelink(shadowqueuedir+"/driver", "../../../../bus/ap/drivers/cex4queue"); err != nil {
			break
		}
		if err = makelink(shadowqueuedir+"/subsystem", "../../../../bus/ap"); err != nil {
			break
		}

		// shadow sys/bus/ap/devices
		shadowapdevicesdir := shadowapbusdir + "/devices"
		if err = makedir(shadowapdevicesdir); err != nil {
			break
		}
		// card link
		linksrc := fmt.Sprintf("%s/%s", shadowapdevicesdir, carddir)
		linkdst := fmt.Sprintf("../../../devices/ap/%s", carddir)
		if err = makelink(linksrc, linkdst); err != nil {
			break
		}
		// queue link
		linksrc = fmt.Sprintf("%s/%s", shadowapdevicesdir, queuedir)
		linkdst = fmt.Sprintf("../../../devices/ap/%s/%s", carddir, queuedir)
		if err = makelink(linksrc, linkdst); err != nil {
			break
		}

		// shadow sys/bus/ap/drivers
		if err = makedir(shadowapbusdir + "/drivers"); err != nil {
			break
		}
		if err = makedir(shadowapbusdir + "/drivers/cex4card"); err != nil {
			break
		}
		linksrc = fmt.Sprintf("%s/drivers/cex4card/%s", shadowapbusdir, carddir)
		linkdst = fmt.Sprintf("../../../../devices/ap/%s", carddir)
		if err = makelink(linksrc, linkdst); err != nil {
			break
		}
		if err = makedir(shadowapbusdir + "/drivers/cex4queue"); err != nil {
			break
		}
		linksrc = fmt.Sprintf("%s/drivers/cex4queue/%s", shadowapbusdir, queuedir)
		linkdst = fmt.Sprintf("../../../../devices/ap/%s/%s", carddir, queuedir)
		if err = makelink(linksrc, linkdst); err != nil {
			break
		}

		// all good, return with the values of the two shadow dirs which are to
		// be used as /sys/bus/ap and /sys/devices/ap within the container
		log.Printf("Shadowsysfs: shadow dir %s created\n", shadowdir)
		return shadowapbusdir, shadowapdevsdir, nil
	}

	// only reached on error
	os.RemoveAll(shadowdir)
	return "", "", err
}

func shadowFetchActiveShadows() ([]string, error) {

	var shadowdirs []string

	_, err := os.Stat(shadowbasedir)
	if err != nil && os.IsNotExist(err) {
		return shadowdirs, nil
	}

	files, err := os.ReadDir(shadowbasedir)
	if err != nil {
		log.Printf("Shadowsysfs: Can't read directory %s: %s\n", shadowbasedir, err)
		return nil, fmt.Errorf("Shadowsysfs: Can't read directory %s: %s", shadowbasedir, err)
	}

	for _, f := range files {
		match, _ := regexp.MatchString("sysfs-apqn-.*", f.Name())
		if match {
			shadowdirs = append(shadowdirs, f.Name())
		}
	}

	return shadowdirs, nil
}

func delShadowSysfs(shadowdir string) {

	dir := fmt.Sprintf("%s/%s", shadowbasedir, shadowdir)
	os.RemoveAll(dir)
}

func addLiveMounts(id string, carsp *kdp.ContainerAllocateResponse, card, queue int) error {

	makelink := func(src, dst string) error {
		//fmt.Printf("debug: makelink(%s,%s)\n", src, dst)
		err := os.Symlink(dst, src)
		if err != nil {
			log.Printf("Shadowsysfs: Failed to create symlink %s -> %s: %s\n", src, dst, err)
			return fmt.Errorf("Shadowsysfs: Failed to create symlink %s -> %s: %s", src, dst, err)
		}
		return nil
	}

	shadowdir := fmt.Sprintf("%s/sysfs-%s", shadowbasedir, id)
	apcarddir := fmt.Sprintf("%s/card%02x", apdevsdir, card)
	apqueuedir := fmt.Sprintf("%s/%02x.%04x", apcarddir, card, queue)

	// Create symlink from original ap queue dir to tmp_bus dir in shadowsysfs
	linkdst := fmt.Sprintf("%s", apqueuedir)
	linksrc := fmt.Sprintf("%s/tmp_bus", shadowdir)
	if err := makelink(linksrc, linkdst); err != nil {
		log.Printf("Shadowsysfs: Error makelink: %s --> %s\n", linkdst, linksrc)
		return fmt.Errorf("Shadowsysfs: Failed to create directory symlink from %s to %s/tmp_bus", apqueuedir, shadowdir)
	}

	// Over-mount container dir with tmp_bus dir in shadowsysfs
	container_path := fmt.Sprintf("%s/devices/%02x.%04x", apbusdir, card, queue)
	host_path := fmt.Sprintf("%s/tmp_bus", shadowdir)
	carsp.Mounts = append(carsp.Mounts, &kdp.Mount{
		ContainerPath: container_path,
		HostPath:      host_path,
		ReadOnly:      true})

	log.Printf("Shadowsysfs: Container has now live access to host's %s.\n", apqueuedir)
	log.Printf("Shadowsysfs: Files in %s are symlinked to %s.\n", apcarddir, apqueuedir)

	return nil
}
