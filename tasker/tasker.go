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
	"sync"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
)

var (
	iCh, tCh chan *task
	quitCh   chan struct{}
	wg       sync.WaitGroup
)

func init() {
	iCh = make(chan *task)
	tCh = make(chan *task)
	quitCh = make(chan struct{})
	go tasker()
	go mover()
}

type task struct {
	name string
	run  func()
}

// Enqueue adds a task to the task queue.
// Calls to Enqueue after a Close will block.
func Enqueue(name string, f func()) {
	iCh <- &task{name: name, run: f}
}

// mover reads tasks from iCh, buffers them internally in q and writes them to tCh.
// When it receives a message on quitCh, it stops reading tasks from iCh, runs until all tasks
// from q have been accepted by tCh, and exits, closing tCh.
func mover() {
	// Makes the tasker() thread exit when quitCh receives a message and there are no more tasks in q.
	defer close(tCh)
	// The task queue.
	q := make([]*task, 0)
	// Refers to iCh until a message arrives on quitCh, when it is set to it nil.
	rdCh := iCh
	// Refers to tCh iff there are tasks to send. Nil otherwise.
	var wrCh chan *task
	// The next task to send on tCh or nil
	var nt *task
	for rdCh != nil || wrCh != nil {
		// The select statement below relies on nil channels always blocking.
		select {
		case t := <-rdCh:
			q = append(q, t)
			wrCh = tCh
			nt = q[0]
		case wrCh <- nt:
			q[0] = nil
			q = q[1:]
			if len(q) == 0 {
				wrCh = nil
				nt = nil
			} else {
				nt = q[0]
			}
		case <-quitCh:
			{
				rdCh = nil
			}
		}
	}
}

// Close prevents any further tasks from being enqueued and waits for the queue to empty.
// Subsequent calls to Close() will block.
func Close() {
	quitCh <- struct{}{}
	wg.Wait()
}

func tasker() {
	wg.Add(1)
	defer wg.Done()
	logger.Debugf("Waiting for tasks to run.")
	for t := range tCh {
		logger.Debugf("Tasker running %q.", t.name)
		t.run()
		logger.Debugf("Finished task %q.", t.name)
	}
	logger.Debugf("Tasker exiting.")
}
