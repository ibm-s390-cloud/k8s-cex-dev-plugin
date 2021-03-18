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
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
)

const (
	apsysfsdir     = "/sys/bus/ap"
	apsysfsdevsdir = "/sys/devices/ap"
	// Estimate how much space one APQN requires when printing
	apqnstringestimate = 6 + 3 + 3 + 4 + 5 + 1
)

type APQN struct {
	adapter int
	domain  int
	gen     string // something like "cex7"
	mode    string // mode string "ep11" or "cca" or "accel"
	online  bool   // true = online, false = offline
}

func (a *APQN) String() string {
	return fmt.Sprintf("(%d,%d,%s,%s,%v)", a.adapter, a.domain, a.gen, a.mode, a.online)
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

func (l APQNList) Len() int {
	return len(l)
}

func (l APQNList) Less(i, j int) bool {
	return l[i].mode[0] < l[j].mode[0]
}

func (l APQNList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l APQNList) filterMode(mode string) APQNList {
	sort.Sort(l)
	start, end := 0, len(l)
	for ; start < end; start++ {
		if l[start].mode == mode {
			break
		}
	}
	for ; end > start; end-- {
		if l[end-1].mode == mode {
			break
		}
	}
	return l[start:end]
}

func apHasApSupport() bool {

	_, err := os.Stat(apsysfsdir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("ap: No AP bus support (AP bus sysfs dir does not exit)\n")
		} else {
			log.Printf("ap: Error reading AP bus sysfs dir: %s\n", err)
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
		log.Printf("ap: Error reading 'online' file from queudir '%s': %s\n", carddir, err)
		return nil, fmt.Errorf("ap: Error reading 'online' file from queudir '%s': %w\n", carddir, err)
	}

	a := new(APQN)
	a.adapter = card
	a.domain = queue
	if online[0] == '1' {
		a.online = true
	}

	//fmt.Printf("debug: apScanQueueDir apqn=%v\n", a)
	return a, nil
}

func apScanCardDir(carddir string) (APQNList, error) {

	var apqns APQNList

	files, err := ioutil.ReadDir(apsysfsdevsdir + "/" + carddir)
	if err != nil {
		log.Printf("ap: Error reading card directory '%s': %s\n", carddir, err)
		return nil, fmt.Errorf("ap: Error reading card directory '%s': %w\n", carddir, err)
	}

	cardtype, err := apReadFirstLineFromFile(apsysfsdevsdir + "/" + carddir + "/" + "type")
	if err != nil {
		log.Printf("ap: Error reading 'type' file from card directory '%s': %s\n", carddir, err)
		return nil, fmt.Errorf("ap: Error reading 'type' file from card directory '%s': %w\n", carddir, err)
	}
	match, _ := regexp.MatchString("CEX[[:digit:]]+[ACP]", cardtype)
	if !match {
		log.Printf("ap: Error matching cardtype '%s' from card directory '%s'\n", cardtype, carddir)
		return nil, fmt.Errorf("ap: Error matching cardtype '%s' from card directory '%s'\n", cardtype, carddir)
	}
	var cardgen int
	var cardmode byte
	n, err := fmt.Sscanf(cardtype, "CEX%d%c", &cardgen, &cardmode)
	if err != nil || n != 2 {
		log.Printf("ap: Error parsing cardtype string '%s' from card directory '%s'\n", cardtype, carddir)
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
		a.gen = cgen
		a.mode = cmode
		apqns = append(apqns, a)
	}

	//fmt.Printf("debug: apScanCardDir apqns=%s\n", apqnsAsString(apqns))
	return apqns, nil
}

func apScanAPQNs() (APQNList, error) {

	var apqns APQNList

	files, err := ioutil.ReadDir(apsysfsdevsdir)
	if err != nil {
		log.Printf("ap: Error reading AP devices sysfs dir: %s\n", err)
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

	log.Printf("ap: apScanAPQNs() found %d APQNs: %s\n", len(apqns), apqns)

	return apqns, nil
}
