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

// Package tasker is a task queue for the osconfig_agent.
package tasker

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

type timeNote struct {
	id int
	timestamp time.Time
}

var notes = []*timeNote{}
var lock sync.Mutex
var counter int


// TestEnqueue_taskRunSequentially creates task that writes
// to a common file
func TestEnqueue_taskRunSequentially(t *testing.T) {
	times := 100
	for i := 0; i < times; i++ {
		 AddToQueue()
	}
	Close()

	for i := 1; i < times; i++ {
		if notes[i].timestamp.Sub(notes[i-1].timestamp) < 0 {
			t.Errorf("task(%d) expected to run earlier\n", i)
		}
	}
}

func AddToQueue() {
	lock.Lock()
	counter++
	i := counter
	lock.Unlock()
	Enqueue(strconv.Itoa(i), func() {
		notes = append(notes, &timeNote{id:i, timestamp:time.Now()})
	})
}