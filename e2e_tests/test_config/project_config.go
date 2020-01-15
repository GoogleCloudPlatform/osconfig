//  Copyright 2019 Google Inc. All Rights Reserved.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package testconfig

import (
	"math/rand"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/config"
)

// Project is details of test Project.
type Project struct {
	sync.Mutex

	TestProjectID        string
	ServiceAccountEmail  string
	ServiceAccountScopes []string
	testZones            map[string]int
	zoneIndices          []string                
}

var mx sync.Mutex
var projects = make(map[string]*Project)

// GetProject creates a test Project to be used.
func GetProject() *Project {
	projectIDs := config.Projects()
	projectID := projectIDs[rand.Intn(len(projectIDs))]
	mx.Lock()
	defer mx.Unlock()
	p, ok := projects[projectID]
	if ok {
		return p
	}

	testZones := map[string]int{}
	var zoneIndices []string
	for k,v :=range config.Zones() {
		testZones[k] = v
		zoneIndices = append(zoneIndices, k)
	}

	p = &Project{
		TestProjectID:       projectID,
		testZones:           testZones,
		zoneIndices:         zoneIndices,
		ServiceAccountEmail: "default",
		ServiceAccountScopes: []string{
			"https://www.googleapis.com/auth/cloud-platform",
			"https://www.googleapis.com/auth/devstorage.full_control",
		},
	}

	projects[projectID] = p
	return p
}

// AcquireZone returns a random zone that still has capacity, or waits until there is one.
func (p *Project) AcquireZone() string {
	timer := time.NewTimer(30 * time.Minute)
	for {
		p.Lock()

		zc := len(p.zoneIndices)
		if zc == 0 {
			p.Unlock()
			select {
			case <-timer.C:
				return "Not enough zone quota sepcified. Specify additional quota in `test_zones`."
			default:
				time.Sleep(10 * time.Second)
				continue
			}
		}

		// Pick a random zone.
		zi := rand.Intn(zc)
		z := p.zoneIndices[zi]

		// Decrement the number of instances that this zone can host.
		p.testZones[z]--
		// Remove this zone from zoneIndices if it can't host any more instances.
		if p.testZones[z] == 0 {
			p.zoneIndices = append(p.zoneIndices[:zi], p.zoneIndices[zi+1:]...)
		}

		p.Unlock()
		return z
	}
}

// ReleaseZone returns a zone so other tests can use it.
func (p *Project) ReleaseZone(z string) {
	p.Lock()
	defer p.Unlock()

	n, ok := p.testZones[z]
	if !ok {
		// This shouldn't happen, but if it does just ignore it.
		return
	}
	if n == 0 {
		p.zoneIndices = append(p.zoneIndices, z)
	}

	p.testZones[z]++
}
