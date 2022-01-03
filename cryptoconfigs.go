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
 * functions related to CEX crypto configuration
 */

package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// configuration file name
	configfile = "/config/cex_resources.json"
)

var (
	mu   sync.RWMutex
	cc   *CryptoConfig
	tag  []byte
	tick *time.Ticker
)

var cccheckinterval = time.Duration(getenvint("CRYPTOCONFIG_CHECK_INTERVAL", 120, 120))

type CryptoConfig struct {
	CryptoConfigSets []*CryptoConfigSet `json:"cryptoconfigsets"`
}

type CryptoConfigSet struct {
	SetName   string    `json:"setname"`
	Project   string    `json:"project"`
	CexMode   string    `json:"cexmode"`
	MinCexGen string    `json:"mincexgen"`
	APQNDefs  []APQNDef `json:"apqns"`
}

type APQNDef struct {
	Adapter   int    `json:"adapter"`
	Domain    int    `json:"domain"`
	MachineId string `json:"machineid"`
}

func (cc CryptoConfig) String() string {

	var b strings.Builder
	b.WriteString("CryptoConfig[")
	for i, e := range cc.CryptoConfigSets {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, "%s", e)
	}
	b.WriteString("]")
	return b.String()
}

func (cc CryptoConfig) Equal(o *CryptoConfig) bool {

	if len(cc.CryptoConfigSets) != len(o.CryptoConfigSets) {
		return false
	}
	for _, s1 := range cc.CryptoConfigSets {
		found := false
		for _, s2 := range o.CryptoConfigSets {
			if s1.SetName == s2.SetName {
				if !s1.equal(s2) {
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

func (cc CryptoConfig) Verify() bool {

	var checkapqns func(APQNDef, APQNDef) bool = func(a1, a2 APQNDef) bool {
		if a1.Adapter == a2.Adapter && a1.Domain == a2.Domain {
			if a1.MachineId == a2.MachineId ||
				len(a1.MachineId) == 0 ||
				len(a2.MachineId) == 0 {
				return false
			}
		}
		return true
	}

	for i, s := range cc.CryptoConfigSets {
		prestr := "CryptoConfig verify:"
		// check setnames - need to be unique
		for j, s2 := range cc.CryptoConfigSets {
			if i != j && s.SetName == s2.SetName {
				log.Printf("%s More than one set '%s' - setname needs to be unique\n", prestr, s.SetName)
				return false
			}
		}
		prestr = fmt.Sprintf("CryotoConfigSet '%s' verify:", s.SetName)
		// check projectname - must not be empty
		if len(s.Project) == 0 {
			log.Printf("%s Projectname is empty\n", prestr)
			return false
		}
		// check cexmode
		if len(s.CexMode) > 0 {
			switch s.CexMode {
			case "ep11", "cca", "accel":
				break
			default:
				log.Printf("%s Unknown/unsupported cexmode '%s'\n", prestr, s.CexMode)
				return false
			}
		}
		// check mincexgen
		if len(s.MinCexGen) > 0 {
			match, _ := regexp.MatchString("cex[56789]", s.MinCexGen)
			if !match {
				log.Printf("%s Unknown/unsupported mincexgen '%s'\n", prestr, s.MinCexGen)
				return false
			}
		}
		// check APQNDefs
		for k, a := range s.APQNDefs {
			// check APQN adapter value
			if a.Adapter < 0 || a.Adapter > 255 {
				log.Printf("%s APQN(%d,%d) - invalid adapter %d [0...255]\n",
					prestr, a.Adapter, a.Domain, a.Adapter)
				return false
			}
			// check APQN domain value
			if a.Domain < 0 || a.Domain > 255 {
				log.Printf("%s APQN(%d,%d) - invalid domain %d [0...255]\n",
					prestr, a.Adapter, a.Domain, a.Domain)
				return false
			}
			// each APQN neads to be unique within the configset
			for n, a2 := range s.APQNDefs {
				if k != n {
					if !checkapqns(a, a2) {
						log.Printf("%s APQN(%d,%d) and APQN(%d,%d) are effectively the same\n",
							prestr, a.Adapter, a.Domain, a2.Adapter, a2.Domain)
						return false
					}
				}
			}
			// and must not appear on other config sets
			for j, s2 := range cc.CryptoConfigSets {
				if i != j {
					for _, a2 := range s2.APQNDefs {
						if !checkapqns(a, a2) {
							log.Printf("%s APQN(%d,%d) appears also in set '%s'\n",
								prestr, a.Adapter, a.Domain, s2.SetName)
							return false
						}
					}
				}
			}
		}
	}

	return true
}

func (cc CryptoConfig) PrettyLog() {

	n := len(cc.CryptoConfigSets)
	log.Printf("CryptoConfig (%d CryptoConfigSets):\n", n)
	for _, e := range cc.CryptoConfigSets {
		log.Printf("  setname: '%s'\n", e.SetName)
		log.Printf("    project: '%s'\n", e.Project)
		if len(e.CexMode) > 0 {
			log.Printf("    cexmode: '%s'\n", e.CexMode)
		}
		if len(e.MinCexGen) > 0 {
			log.Printf("    mincexgen: '%s'\n", e.MinCexGen)
		}
		n = len(e.APQNDefs)
		if n > 0 {
			log.Printf("    %d equvialent APQNs:\n", n)
			for _, a := range e.APQNDefs {
				midstr := a.MachineId
				if len(midstr) == 0 {
					midstr = "*"
				}
				log.Printf("      APQN adapter=%d domain=%d machineid='%s'\n",
					a.Adapter, a.Domain, midstr)
			}
		} else {
			log.Printf("    no equvialent APQNs defined\n")
		}
	}
}

func (cc *CryptoConfig) GetListOfSetNames() []string {

	var sets []string

	if cc != nil {
		for _, s := range cc.CryptoConfigSets {
			sets = append(sets, s.SetName)
		}
	}

	return sets
}

func (cc *CryptoConfig) GetCryptoConfigSet(setname string) *CryptoConfigSet {

	if cc != nil {
		for _, s := range cc.CryptoConfigSets {
			if s.SetName == setname {
				return s
			}
		}
	}

	return nil
}

func (cc *CryptoConfig) GetCryptoConfigSetForThisAPQN(ap, dom int, machineid string) *CryptoConfigSet {

	if cc != nil {
		for _, s := range cc.CryptoConfigSets {
			for _, a := range s.APQNDefs {
				if a.Adapter == ap && a.Domain == dom {
					if len(a.MachineId) > 0 && a.MachineId != machineid {
						continue
					}
					return s
				}
			}
		}
	}

	return nil
}

func (s CryptoConfigSet) String() string {
	return fmt.Sprintf("Set(setname=%s,project=%s,cexmode=%s,mincexgen=%s,apqndefs=%s)",
		s.SetName, s.Project, s.CexMode, s.MinCexGen, s.APQNDefs)
}

func (s CryptoConfigSet) equal(o *CryptoConfigSet) bool {

	if s.SetName != o.SetName ||
		s.Project != o.Project ||
		s.CexMode != o.CexMode ||
		s.MinCexGen != o.MinCexGen ||
		len(s.APQNDefs) != len(o.APQNDefs) {
		return false
	}
	for _, a := range s.APQNDefs {
		found := false
		for _, o := range o.APQNDefs {
			if a == o {
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

func (a APQNDef) String() string {
	return fmt.Sprintf("APQN(%d,%d,%s)", a.Adapter, a.Domain, a.MachineId)
}

func ccReadConfigFile() (*CryptoConfig, error) {

	var cc CryptoConfig

	config, err := os.Open(configfile)
	if err != nil {
		log.Printf("CryptoConfig: Can't open config file '%s': %s\n", configfile, err)
		return nil, fmt.Errorf("CryptoConfig: Can't open config file '%s': %w", configfile, err)
	}
	defer config.Close()

	rawdata, err := ioutil.ReadAll(config)
	if err != nil {
		log.Printf("CryptoConfig: Error reading config file '%s': %s\n", configfile, err)
		return nil, fmt.Errorf("CryptoConfig: Error reading config file '%s': %w", configfile, err)
	}

	if err = json.Unmarshal(rawdata, &cc); err != nil {
		log.Printf("CryptoConfig: Error parsing config file '%s': %s\n", configfile, err)
		return nil, fmt.Errorf("CryptoConfig: Error parsing config file '%s': %w", configfile, err)
	}

	return &cc, nil
}

func ccGetMachineId() (string, error) {

	machineid := ""

	// use /proc/sysinfo
	sysinfofile := "/proc/sysinfo"
	sysinfo, err := os.Open(sysinfofile)
	if err != nil {
		log.Printf("CryptoConfig: Can't open sysinfo file '%s': %s\n", sysinfofile, err)
		return "", fmt.Errorf("CryptoConfig: Can't open sysinfo file '%s': %w", sysinfofile, err)
	}
	defer sysinfo.Close()

	manufacturer := ""
	reManufacturer := regexp.MustCompile("Manufacturer:[[:space:]]+(.+)")
	machinetype := ""
	reMachinetype := regexp.MustCompile("Type:[[:space:]]+(.+)")
	sequencecode := ""
	reSequencecode := regexp.MustCompile("Sequence Code:[[:space:]]+(.+)")

	r := bufio.NewReader(sysinfo)
	for {
		line, err := r.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("CryptoConfig: Error reading sysinfo file '%s': %s\n", sysinfofile, err)
			return "", fmt.Errorf("CryptoConfig: Error reading sysinfo file '%s': %w", sysinfofile, err)
		}
		str := strings.TrimSpace(line)
		if len(manufacturer) == 0 {
			match := reManufacturer.FindStringSubmatch(str)
			if match != nil {
				manufacturer = match[1]
			}
		}
		match := reManufacturer.FindStringSubmatch(str)
		if match != nil {
			manufacturer = match[1]
		}
		match = reMachinetype.FindStringSubmatch(str)
		if match != nil {
			machinetype = match[1]
		}
		match = reSequencecode.FindStringSubmatch(str)
		if match != nil {
			sequencecode = match[1]
		}
	}
	//fmt.Printf("Manufacturer=%s\n", manufacturer)
	//fmt.Printf("Machinetype=%s\n", machinetype)
	//fmt.Printf("Sequencecode=%s\n", sequencecode)
	if len(manufacturer) == 0 || len(machinetype) == 0 || len(sequencecode) == 0 {
		log.Printf("CryptoConfig: Error extracting fields 'Manufacturer', 'Type' and 'Sequence Code' from sysinfo\n")
		return "", fmt.Errorf("CryptoConfig: Error extracting fields 'Manufacturer', 'Type' and 'Sequence Code' from sysinfo")
	}
	machineid = manufacturer + "-" + machinetype + "-" + sequencecode

	return machineid, nil
}

func ccGetTag() ([]byte, error) {
	config, err := os.Open(configfile)
	if err != nil {
		log.Printf("CryptoConfig: Can't open config file '%s': %s\n", configfile, err)
		return nil, fmt.Errorf("CryptoConfig: Can't open config file '%s': %w", configfile, err)
	}
	defer config.Close()
	h := sha256.New()
	if _, err := io.Copy(h, config); err != nil {
		return nil, fmt.Errorf("Could not generate tag for configuration file '%s': %w", configfile, err)
	}
	return h.Sum(nil), nil
}

func updateConfig() error {
	newtag, err := ccGetTag()
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if bytes.Equal(newtag, tag) {
		return nil
	}
	log.Printf("CryptoConfig: Configuration changes detected\n")
	// In case of an error, do not provide any configuration.
	// If reading and verification succeeds, we will overwrite this below
	cc, tag = nil, nil
	newcc, err := ccReadConfigFile()
	if err != nil {
		return fmt.Errorf("CryptoConfig: Failed to read or parse the new configuration!")
	}
	if !newcc.Verify() {
		return fmt.Errorf("CryptoConfig: Failed to verify new configuration!")
	}
	cc, tag = newcc, newtag
	log.Printf("CryptoConfig: Configuration successful updated\n")
	return nil
}

func InitializeConfigWatcher() (*CryptoConfig, error) {
	err := updateConfig()
	if err != nil {
		return nil, err
	}
	tick = time.NewTicker(cccheckinterval * time.Second)
	go func() {
		for {
			t := <-tick.C
			if t.IsZero() {
				// Uninitialized time => tick just got closed
				return
			}
			err := updateConfig()
			if err != nil {
				log.Printf("CryptoConfig: Failed to update config: %s\n", err)
			}
		}
	}()
	return cc, nil
}

func StopConfigWatcher() {
	tick.Stop()
}

func GetCurrentCryptoConfig() *CryptoConfig {
	mu.RLock()
	defer mu.RUnlock()
	return cc
}

func GetCurrentCryptoConfigSet(ccset *CryptoConfigSet, resource string, cctag []byte) (*CryptoConfigSet, []byte) {
	mu.RLock()
	defer mu.RUnlock()
	if bytes.Equal(cctag, tag) {
		return ccset, cctag
	}
	return cc.GetCryptoConfigSet(resource), tag
}
