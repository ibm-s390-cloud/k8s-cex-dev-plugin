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
 * main function
 */

package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	cextypeenvname = "CEXTYPE" // environment variable for the cextype
	ticktime       = 5         // seconds
)

// main

func main() {

	var cextype string

	log.SetPrefix("Zcrypt-dev-plugin: ")
	log.Println("S390 zcrypt k8s device plugin starting")

	flag.StringVar(&cextype, "t", "ep11", "Set cextype - one of 'ep11', 'cca' or 'accel'")
	flag.Parse()

	cextypeenv := os.Getenv(cextypeenvname)
	if len(cextypeenv) > 0 {
		cextype = cextypeenv
		log.Printf("%s='%s' environment variable found\n", cextypeenvname, cextype)
	}

	switch cextype {
	case "ep11", "cca", "accel":
		break
	default:
		log.Fatalf("Unknown/unsupported cextype '%s'\n", cextype)
	}
	if !apHasApSupport() {
		log.Fatalf("No ap bus support available\n")
	}
	if !zcryptHasNodesSupport() {
		log.Fatalf("No zcrypt multiple node support available\n")
	}

	zcdevplugin, err := NewZcryptDevPlugin(cextype)
	if err != nil {
		log.Fatalf("main: NewZcryptDevPlugin() failed: %s\n", err)
	}
	err = zcdevplugin.Start()
	if err != nil {
		log.Fatalf("main: plugin start failed: %s\n", err)
	}

	tickdelay := ticktime * time.Second
	ticker := time.NewTicker(tickdelay)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

ForLoop:
	for {
		select {
		case <-ticker.C:
			if zcdevplugin == nil {
				log.Println("main: trying to reestablish plugin...")
				zcdevplugin, err = NewZcryptDevPlugin(cextype)
				if err != nil {
					log.Println("main: create of plugin failed, will try again later")
					break
				}
				if err := zcdevplugin.Start(); err != nil {
					log.Println("main: start of plugin failed, will try again later")
					zcdevplugin = nil
					break
				}
				log.Println("main: plugin successful reestablished")
			} else {
				if k8check := checkK8DevicePluginPath(); !k8check {
					log.Println("main: k8check failed, restarting plugin...")
					zcdevplugin.Stop()
					zcdevplugin = nil
				}
			}
		case sig := <-sigChan:
			switch sig {
			case syscall.SIGHUP:
				log.Println("main: Signal 'SIGHUP' received, restarting plugin...")
				if zcdevplugin != nil {
					zcdevplugin.Stop()
				}
				zcdevplugin = nil
			default:
				log.Printf("main: Signal '%v' received, terminating...\n", sig)
				if zcdevplugin != nil {
					zcdevplugin.Stop()
				}
				break ForLoop
			}
		}
	}

	ticker.Stop()

	log.Println("S390 zcrypt k8s device plugin exit")
}
