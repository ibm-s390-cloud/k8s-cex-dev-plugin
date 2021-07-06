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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
)

const (
	// configuration file name
	configfile = "cex_resources.json"
)

type CryptoConfig struct {
	CryptoConfigSets []*CryptoConfigSet `json:"cryptoconfigsets"`
}

type CryptoConfigSet struct {
	SetName   string    `json:"setname"`
	Project   string    `json:"project"`
	CexType   string    `json:"cextype"`
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
		// check cextype
		if len(s.CexType) > 0 {
			switch s.CexType {
			case "ep11", "cca", "accel":
				break
			default:
				log.Printf("%s Unknown/unsupported cextype '%s'\n", prestr, s.CexType)
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
		if len(e.CexType) > 0 {
			log.Printf("    cextype: '%s'\n", e.CexType)
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

func (cc CryptoConfig) GetListOfSetNames() []string {

	var sets []string

	for _, s := range cc.CryptoConfigSets {
		sets = append(sets, s.SetName)
	}

	return sets
}

func (cc CryptoConfig) GetCryptoConfigSet(setname string) *CryptoConfigSet {

	for _, s := range cc.CryptoConfigSets {
		if s.SetName == setname {
			return s
		}
	}

	return nil
}

func (s CryptoConfigSet) String() string {
	return fmt.Sprintf("Set(setname=%s,project=%s,cextype=%s,mincexgen=%s,apqndefs=%s)",
		s.SetName, s.Project, s.CexType, s.MinCexGen, s.APQNDefs)
}

func (s CryptoConfigSet) equal(o *CryptoConfigSet) bool {

	if s.SetName != o.SetName ||
		s.Project != o.Project ||
		s.CexType != o.CexType ||
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
		return nil, fmt.Errorf("CryptoConfig: Can't open config file '%s': %w\n", configfile, err)
	}
	defer config.Close()

	rawdata, err := ioutil.ReadAll(config)
	if err != nil {
		log.Printf("CryptoConfig: Error reading config file '%s': %s\n", configfile, err)
		return nil, fmt.Errorf("CryptoConfig: Error reading config file '%s': %w\n", configfile, err)
	}

	if err = json.Unmarshal(rawdata, &cc); err != nil {
		log.Printf("CryptoConfig: Error parsing config file '%s': %s\n", configfile, err)
		return nil, fmt.Errorf("CryptoConfig: Error parsing config file '%s': %w\n", configfile, err)
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
		return "", fmt.Errorf("CryptoConfig: Can't open sysinfo file '%s': %w\n", sysinfofile, err)
	}
	defer sysinfo.Close()

	manufactorer := ""
	reManufactorer := regexp.MustCompile("Manufacturer:[[:space:]]+(.+)")
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
			return "", fmt.Errorf("CryptoConfig: Error reading sysinfo file '%s': %w\n", sysinfofile, err)
		}
		str := strings.TrimSpace(line)
		if len(manufactorer) == 0 {
			match := reManufactorer.FindStringSubmatch(str)
			if match != nil {
				manufactorer = match[1]
			}
		}
		match := reManufactorer.FindStringSubmatch(str)
		if match != nil {
			manufactorer = match[1]
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
	//fmt.Printf("Manufactorer=%s\n", manufactorer)
	//fmt.Printf("Machinetype=%s\n", machinetype)
	//fmt.Printf("Sequencecode=%s\n", sequencecode)
	if len(manufactorer) == 0 || len(machinetype) == 0 || len(sequencecode) == 0 {
		log.Printf("CryptoConfig: Error extracting fields 'Manufactorer', 'Type' and 'Sequence Code' from sysinfo\n")
		return "", fmt.Errorf("CryptoConfig: Error extracting fields 'Manufactorer', 'Type' and 'Sequence Code' from sysinfo\n")
	}
	machineid = manufactorer + "-" + machinetype + "-" + sequencecode

	return machineid, nil
}