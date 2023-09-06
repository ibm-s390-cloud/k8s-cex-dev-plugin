/*
 * Copyright 2022 IBM Corp.
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
 * Author(s): Juergen Christ <jchrist@linux.ibm.com>
 *
 * s390 zcrypt kubernetes device plugin
 * functions related to CEX crypto configuration
 */

// run with
// $ go test cryptoconfigs*.go
// or for more verbose output
// $ go test -v cryptoconfigs*.go
// or for coverage
// $ go test -coverprofile=c.out cryptoconfigs*.go; go tool cover -html=c.out

package main

import (
	"log"
	"os"
	"strconv"
	"testing"
)

type TestAPQN struct {
	adapter   int
	domain    int
	machineid string
	setidx    int
}

// Have to provide this to remove some dependencies.
func getenvint(envvar string, defaultval, minval, maxval int) int {
	valstr, isset := os.LookupEnv(envvar)
	if isset {
		valint, err := strconv.Atoi(valstr)
		if err != nil {
			log.Printf("Invalid setting for %s: %q.  Using default value...\n", envvar, err)
			return defaultval
		}
		if valint < minval {
			return minval
		}
		if valint > maxval {
			return maxval
		}
		return valint
	}
	return defaultval
}

func Int(v int) *int { return &v }

func TestCryptoConfigVerification(t *testing.T) {
	var tests = []struct {
		config CryptoConfig
		name   string
		want   bool
	}{
		// non-unique set name
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "set",
						Project: "test",
					},
					&CryptoConfigSet{
						SetName: "set",
						Project: "test",
					},
				},
			},
			name: "non-unique set names",
			want: false,
		},
		// empty set name
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "",
						Project: "test",
					},
				},
			},
			name: "empty set name",
			want: false,
		},
		// omitted set name
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						Project: "test",
					},
				},
			},
			name: "omitted set name",
			want: false,
		},
		// empty project name
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "set",
						Project: "",
					},
				},
			},
			name: "empty project name",
			want: false,
		},
		// omitted project name
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "set",
					},
				},
			},
			name: "omitted project name",
			want: false,
		},
		// invalid cex-mode
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "set",
						Project: "test",
						CexMode: "ignored",
					},
				},
			},
			name: "invalid cex-mode",
			want: false,
		},
		// invalid mincexgen
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName:   "set",
						Project:   "test",
						MinCexGen: "cex456789",
					},
				},
			},
			name: "invalid min cex gen",
			want: false,
		},
		// invalid adapter
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "set",
						Project: "test",
						APQNDefs: []APQNDef{
							APQNDef{
								Adapter: -1,
								Domain:  0,
							},
						},
					},
				},
			},
			name: "negative adapter",
			want: false,
		},
		// livesysfs value 1 given
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName:   "set",
						Project:   "test",
						Livesysfs: Int(1),
					},
				},
			},
			name: "valid livesysfs 1 value",
			want: true,
		},
		// livesysfs value 0 given
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName:   "set",
						Project:   "test",
						Livesysfs: Int(0),
					},
				},
			},
			name: "valid livesysfs 0 value",
			want: true,
		},
		// invalid livesysfs value -1 given
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName:   "set",
						Project:   "test",
						Livesysfs: Int(-1),
					},
				},
			},
			name: "invalid livesysfs -1 value",
			want: false,
		},
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "set",
						Project: "test",
						APQNDefs: []APQNDef{
							APQNDef{
								Adapter: 4711,
								Domain:  0,
							},
						},
					},
				},
			},
			name: "huge adapter",
			want: false,
		},
		// invalid domain
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "set",
						Project: "test",
						APQNDefs: []APQNDef{
							APQNDef{
								Adapter: 0,
								Domain:  -1,
							},
						},
					},
				},
			},
			name: "negative domain",
			want: false,
		},
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "set",
						Project: "test",
						APQNDefs: []APQNDef{
							APQNDef{
								Adapter: 0,
								Domain:  4711,
							},
						},
					},
				},
			},
			name: "huge domain",
			want: false,
		},
		// duplicated APQN in set
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "set",
						Project: "test",
						APQNDefs: []APQNDef{
							APQNDef{
								Adapter: 0,
								Domain:  0,
							},
							APQNDef{
								Adapter: 0,
								Domain:  0,
							},
						},
					},
				},
			},
			name: "duplicated APQN",
			want: false,
		},
		// APQN in multiple sets
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "set1",
						Project: "test",
						APQNDefs: []APQNDef{
							APQNDef{
								Adapter: 0,
								Domain:  0,
							},
						},
					},
					&CryptoConfigSet{
						SetName: "set2",
						Project: "test",
						APQNDefs: []APQNDef{
							APQNDef{
								Adapter: 0,
								Domain:  0,
							},
						},
					},
				},
			},
			name: "APQN in multiple sets",
			want: false,
		},
		// everything should be fine...
		{
			config: CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName:    "set1",
						Project:    "test",
						CexMode:    "cca",
						MinCexGen:  "cex7",
						Overcommit: 10,
						APQNDefs: []APQNDef{
							APQNDef{
								Adapter:   0,
								Domain:    0,
								MachineId: "1",
							},
							APQNDef{
								Adapter:   0,
								Domain:    0,
								MachineId: "2",
							},
						},
					},
					&CryptoConfigSet{
						SetName:    "set2",
						Project:    "test",
						CexMode:    "ep11",
						MinCexGen:  "cex6",
						Overcommit: 0,
						APQNDefs: []APQNDef{
							APQNDef{
								Adapter: 0,
								Domain:  2,
							},
						},
					},
					&CryptoConfigSet{
						SetName:    "set3",
						Project:    "other_test",
						CexMode:    "accel",
						MinCexGen:  "cex4",
						Overcommit: 1,
						APQNDefs: []APQNDef{
							APQNDef{
								Adapter: 1,
								Domain:  2,
							},
						},
					},
				},
			},
			name: "everything fine",
			want: true,
		},
	}
	for _, test := range tests {
		if got := test.config.Verify(); got != test.want {
			t.Errorf(`CryptoConfig.Verify for "%s" returned %v`, test.name, got)
		}
	}
}

func equalSliceContentNoOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	unordered := make(map[string]int)
	for _, s := range a {
		unordered[s]++
	}
	for _, s := range b {
		unordered[s]--
	}
	res := true
	for _, v := range unordered {
		if v != 0 {
			res = false
		}
	}
	return res
}

func TestCryptoConfigSetNames(t *testing.T) {
	var tests = []struct {
		cc   *CryptoConfig
		name string
		sets []string
	}{
		{
			cc: &CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "set0",
					},
					&CryptoConfigSet{
						SetName: "set1",
					},
					&CryptoConfigSet{
						SetName: "set2",
					},
					&CryptoConfigSet{
						SetName: "set3",
					},
				},
			},
			name: "four numbered sets",
			sets: []string{"set0", "set1", "set2", "set3"},
		},
		{
			cc: &CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{},
			},
			name: "empty",
			sets: []string{},
		},
		// We allow a nil receiver
		{
			cc:   nil,
			name: "nil",
			sets: []string{},
		},
	}
	for _, test := range tests {
		if got := test.cc.GetListOfSetNames(); !equalSliceContentNoOrder(got, test.sets) {
			t.Errorf(`CryptoConfig.GetListOfSetNames for "%s" returned %q, expected %q`, test.name, got, test.sets)
		}
	}
}

func TestCryptoConfigGetSet(t *testing.T) {
	var tests = []struct {
		cc   *CryptoConfig
		name string
		sets map[string]int
	}{
		{
			cc: &CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "set0",
					},
					&CryptoConfigSet{
						SetName: "set1",
					},
					&CryptoConfigSet{
						SetName: "set2",
					},
					&CryptoConfigSet{
						SetName: "set3",
					},
				},
			},
			name: "four numbered sets",
			sets: map[string]int{
				"set0":        0,
				"set1":        1,
				"set2":        2,
				"set3":        3,
				"notexisting": -1,
			},
		},
		{
			cc: &CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{},
			},
			name: "empty",
			sets: map[string]int{
				"set0": -1,
			},
		},
		// We allow a nil receiver
		{
			cc:   nil,
			name: "nil",
			sets: map[string]int{
				"set0": -1,
			},
		},
	}
	for _, test := range tests {
		for k, v := range test.sets {
			got := test.cc.GetCryptoConfigSet(k)
			if v < 0 {
				if got != nil {
					t.Errorf(`CryptoConfig.GetCryptoConfigSet for "%s" returned non-nil for name "%s"`,
						test.name, k)
				}
			} else if got != test.cc.CryptoConfigSets[v] {
				t.Errorf(`CryptoConfig.GetCryptoConfigSet for "%s" returned wrong set %q for name "%s"`,
					test.name, got, k)
			}
		}
	}
}

func TestCryptoConfigSetForThisAPQN(t *testing.T) {
	var tests = []struct {
		cc    *CryptoConfig
		name  string
		apqns []TestAPQN
	}{
		{
			cc: &CryptoConfig{
				CryptoConfigSets: []*CryptoConfigSet{
					&CryptoConfigSet{
						SetName: "set1",
						Project: "test",
						APQNDefs: []APQNDef{
							APQNDef{
								Adapter:   0,
								Domain:    0,
								MachineId: "1",
							},
							APQNDef{
								Adapter: 0,
								Domain:  1,
							},
						},
					},
					&CryptoConfigSet{
						SetName: "set2",
						Project: "test",
						APQNDefs: []APQNDef{
							APQNDef{
								Adapter:   0,
								Domain:    0,
								MachineId: "2",
							},
						},
					},
				},
			},
			name: "big test",
			apqns: []TestAPQN{
				TestAPQN{
					adapter:   0,
					domain:    0,
					machineid: "1",
					setidx:    0,
				},
				TestAPQN{
					adapter: 0,
					domain:  1,
					setidx:  0,
				},
				TestAPQN{
					adapter:   0,
					domain:    0,
					machineid: "2",
					setidx:    1,
				},
				TestAPQN{
					adapter:   0,
					domain:    0,
					machineid: "3",
					setidx:    -1,
				},
				TestAPQN{
					adapter: 1,
					domain:  0,
					setidx:  -1,
				},
			},
		},
		// We allow a nil receiver
		{
			cc:   nil,
			name: "nil",
			apqns: []TestAPQN{
				TestAPQN{
					adapter:   0,
					domain:    0,
					machineid: "",
					setidx:    -1,
				},
			},
		},
	}
	for _, test := range tests {
		for _, apqn := range test.apqns {
			got := test.cc.GetCryptoConfigSetForThisAPQN(apqn.adapter, apqn.domain, apqn.machineid)
			if apqn.setidx == -1 {
				if got != nil {
					t.Errorf(`CryptoConfig.GetCryptoConfigSetForThisAPQN for "%s" returned non-nil for APQN (%d,%d,%s)`,
						test.name, apqn.adapter, apqn.domain, apqn.machineid)
				}
			} else if got != test.cc.CryptoConfigSets[apqn.setidx] {
				t.Errorf(`CryptoConfig.GetCryptoConfigSetForThisAPQN for "%s" returned wrong set %q for APQN (%d,%d,%s)`,
					test.name, got, apqn.adapter, apqn.domain, apqn.machineid)
			}
		}
	}
}
