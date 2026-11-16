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

package tasker

import (
	"context"
	"strconv"
	"testing"
	"time"
)

var notes []int

// TestEnqueueTaskRunSequentially to set sequential
// execution of tasks in tasker
func TestEnqueueTaskRunSequentially(t *testing.T) {
	times := 10000
	for i := 0; i < times; i++ {
		addToQueue(i)
	}
	Close()

	if len(notes) != times {
		t.Fatalf("len(notes) != times, %d != %d", len(notes), times)
	}
	for i := 1; i < times; i++ {
		if notes[i] < notes[i-1] {
			t.Errorf("task(%d) expected to run earlier", i)
		}
	}
}

func addToQueue(i int) {
	Enqueue(context.Background(), strconv.Itoa(i), func() {
		notes = append(notes, i)
	})
}

// Close must not panic when nothing was ever enqueued: the agent calls it
// on shutdown whether or not a task ran.
func TestCloseWithoutEnqueue(t *testing.T) {
	// Close keeps the mutex held after it returns, so release whatever
	// state an earlier test left and start from an uninitialized queue.
	_ = mx.TryLock() // held by an earlier Close, or free: either way it is ours now
	mx.Unlock()
	tc = nil
	t.Cleanup(func() { mx.Unlock() })

	done := make(chan struct{})
	go func() {
		defer close(done)
		Close()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close() blocked without any enqueued task")
	}
}
