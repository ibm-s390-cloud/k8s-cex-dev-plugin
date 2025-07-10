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
 * AP bus related functions
 */

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
)

const (
	// Estimate how much space an APQN requires when printing
	apqnstringestimate = 6 + 3 + 3 + 4 + 5 + 1
)

var apsysfsdir = getenvstr("APSYSFS_BUSDIR", "/sys/bus/ap")
var apsysfsdevsdir = getenvstr("APSYSFS_DEVSDIR", "/sys/devices/ap")

type APQN struct {
	Adapter int    `json:"adapter"`
	Domain  int    `json:"domain"`
	Gen     string `json:"gen"`    // something like "cex7"
	Mode    string `json:"mode"`   // mode string "ep11" or "cca" or "accel"
	Online  bool   `json:"online"` // true = online, false = offline
}

func (a *APQN) String() string {
	return fmt.Sprintf("(%d,%d,%s,%s,%v)", a.Adapter, a.Domain, a.Gen, a.Mode, a.Online)
}

type APQNList []*APQN

func (l APQNList) String() string {
	var b strings.Builder
	b.Grow(len(l) * (apqnstringestimate + 2))
	for i, e := range l {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s", e)
	}
	return b.String()
}

func apHasApSupport() bool {

	_, err := os.Stat(apsysfsdir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("Ap: No AP bus support (AP bus sysfs dir %s does not exist)\n", apsysfsdir)
		} else {
			log.Printf("Ap: Error reading AP bus sysfs dir %s: %v\n", apsysfsdir, err)
		}
		return false
	}

	return true
}

func apReadFirstLineFromFile(fname string) (string, error) {

	f, err := os.Open(fname)
	if err != nil {
		return "", err
	}
	defer f.Close()
	r := bufio.NewReader(f)
	str, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	str = strings.TrimSpace(str)
	//fmt.Printf("debug: file '%s': '%s'\n", fname, str)

	return str, nil
}

func apScanQueueDir(carddir, queuedir string) (*APQN, error) {

	var card, queue int
	n, err := fmt.Sscanf(queuedir, "%02x.%04x", &card, &queue)
	if err != nil || n != 2 {
		return nil, errors.New(fmt.Sprintf("Error parsing queuedir '%s'", queuedir))
	}

	online, err := apReadFirstLineFromFile(apsysfsdevsdir + "/" + carddir + "/" + queuedir + "/" + "online")
	if err != nil {
		log.Printf("Ap: Error reading 'online' file from queuedir '%s': %s\n", carddir, err)
		return nil, fmt.Errorf("Ap: Error reading 'online' file from queuedir '%s': %w", carddir, err)
	}

	a := new(APQN)
	a.Adapter = card
	a.Domain = queue
	if online[0] == '1' {
		a.Online = true
	}

	//fmt.Printf("debug: apScanQueueDir apqn=%v\n", a)
	return a, nil
}

func apScanCardDir(carddir string) (APQNList, error) {

	var apqns APQNList

	files, err := os.ReadDir(apsysfsdevsdir + "/" + carddir)
	if err != nil {
		log.Printf("Ap: Error reading card directory '%s': %s\n", carddir, err)
		return nil, fmt.Errorf("Ap: Error reading card directory '%s': %w", carddir, err)
	}

	cardtype, err := apReadFirstLineFromFile(apsysfsdevsdir + "/" + carddir + "/" + "type")
	if err != nil {
		log.Printf("Ap: Error reading 'type' file from card directory '%s': %s\n", carddir, err)
		return nil, fmt.Errorf("Ap: Error reading 'type' file from card directory '%s': %w", carddir, err)
	}
	match, _ := regexp.MatchString("CEX[[:digit:]]+[ACP]", cardtype)
	if !match {
		log.Printf("Ap: Error matching cardtype '%s' from card directory '%s'\n", cardtype, carddir)
		return nil, fmt.Errorf("Ap: Error matching cardtype '%s' from card directory '%s'", cardtype, carddir)
	}
	var cardgen int
	var cardmode byte
	n, err := fmt.Sscanf(cardtype, "CEX%d%c", &cardgen, &cardmode)
	if err != nil || n != 2 {
		log.Printf("Ap: Error parsing cardtype string '%s' from card directory '%s'\n", cardtype, carddir)
		return nil, err
	}
	cgen := fmt.Sprintf("cex%d", cardgen)
	cmode := "unknown"
	switch cardmode {
	case 'A':
		cmode = "accel"
	case 'C':
		cmode = "cca"
	case 'P':
		cmode = "ep11"
	}
	//fmt.Printf("debug: cardgen=cex%d cardmode=%c\n", cardgen, cardmode)

	for _, file := range files {
		fname := file.Name()
		match, _ := regexp.MatchString("[[:xdigit:]]{2}\\.[[:xdigit:]]{4}", fname)
		if !match {
			continue
		}
		//fmt.Printf("debug: scaning queuedir %s\n", fname)
		a, err := apScanQueueDir(carddir, fname)
		if err != nil {
			return nil, err
		}
		a.Gen = cgen
		a.Mode = cmode
		apqns = append(apqns, a)
	}

	//fmt.Printf("debug: apScanCardDir apqns=%s\n", apqnsAsString(apqns))
	return apqns, nil
}

func apScanAPQNs(verbose bool) (APQNList, error) {

	var apqns APQNList

	// scan ap bus dirs and fetch available apqns
	files, err := os.ReadDir(apsysfsdevsdir)
	if err != nil {
		log.Printf("Ap: Error reading AP devices sysfs dir: %s\n", err)
		return nil, err
	}
	for _, file := range files {
		fname := file.Name()
		match, _ := regexp.MatchString("card[[:xdigit:]]{2}", fname)
		if !match {
			continue
		}
		//fmt.Printf("debug: scaning carddir %s\n", fname)
		cardapqns, err := apScanCardDir(fname)
		if err != nil {
			return nil, err
		}
		apqns = append(apqns, cardapqns...)
	}

	if verbose {
		log.Printf("Ap: apScanAPQNs() found %d APQNs: %s\n", len(apqns), apqns)
	}

	return apqns, nil
}

func apEqualAPQNLists(l1, l2 APQNList) bool {

	var found bool

	if len(l1) != len(l2) {
		return false
	}

	for _, a1 := range l1 {
		found = false
		for _, a2 := range l2 {
			if a1.Adapter == a2.Adapter && a1.Domain == a2.Domain {
				if a1.Gen != a2.Gen {
					return false
				}
				if a1.Mode != a2.Mode {
					return false
				}
				if a1.Online != a2.Online {
					return false
				}
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func apGetQueueRequestCounter(ap, dom int) (int, error) {

	sysfsqueuedir := fmt.Sprintf("%s/card%02x/%02x.%04x", apsysfsdevsdir, ap, ap, dom)
	rcountstr, err := apReadFirstLineFromFile(sysfsqueuedir + "/request_count")
	if err != nil {
		log.Printf("Ap: Error reading 'request_count' file from queue %02x.%04x: %s\n", ap, dom, err)
		return 0, fmt.Errorf("Ap: Error reading 'request_count' file from queue %02x.%04x: %w\n", ap, dom, err)
	}
	var rcounter int
	if _, err = fmt.Sscanf(rcountstr, "%d", &rcounter); err != nil {
		log.Printf("Ap: Error parsing 'request_count' file from queue %02x.%04x: %s\n", ap, dom, err)
		return 0, fmt.Errorf("Ap: Error parsing 'request_count' file from queue %02x.%04x: %w\n", ap, dom, err)
	}

	return rcounter, nil
}
