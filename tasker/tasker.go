//  Copyright 2018 Google Inc. All Rights Reserved.
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
	"context"
	"sync"

	"github.com/GoogleCloudPlatform/osconfig/clog"
)

var (
	tc chan *task
	wg sync.WaitGroup
	mx sync.Mutex
)

func initTasker(ctx context.Context) {
	tc = make(chan *task)
	go tasker(ctx)
}

type task struct {
	name string
	run  func()
}

// Enqueue adds a task to the task queue.
// Calls to Enqueue after a Close will block.
func Enqueue(ctx context.Context, name string, f func()) {
	mx.Lock()
	if tc == nil {
		initTasker(ctx)
	}
	tc <- &task{name: name, run: f}
	mx.Unlock()
}

// Close prevents any further tasks from being enqueued and waits for the queue to empty.
// Subsequent calls to Close() will block.
func Close() {
	mx.Lock()
	close(tc)
	wg.Wait()
}

func tasker(ctx context.Context) {
	wg.Add(1)
	defer wg.Done()
	for {
		clog.Debugf(ctx, "Waiting for tasks to run.")
		select {
		case t, ok := <-tc:
			// Indicates an empty and closed channel.
			if !ok {
				return
			}
			clog.Debugf(ctx, "Tasker running %q.", t.name)
			t.run()
			clog.Debugf(ctx, "Finished task %q.", t.name)
		}
	}
}
