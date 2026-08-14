//  Copyright 2026 Google LLC
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

// Package scheduler limits concurrent VM allocations across tests.
package scheduler

import (
	"context"
	"fmt"
	"sync"
)

// Scheduler limits concurrent running test VMs.
type Scheduler struct {
	tokens chan struct{}
}

// New creates a scheduler with the specified capacity limit.
func New(capacity int) (*Scheduler, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("scheduler capacity must be positive, got %d", capacity)
	}
	tokens := make(chan struct{}, capacity)
	for range capacity {
		tokens <- struct{}{}
	}
	return &Scheduler{tokens: tokens}, nil
}

// Lease represents an acquired concurrency token.
type Lease struct {
	release func()
	once    sync.Once
}

// Release returns capacity to the scheduler idempotently.
func (l *Lease) Release() {
	if l == nil {
		return
	}
	l.once.Do(l.release)
}

// Acquire waits for an available capacity slot or returns when context is cancelled.
func (s *Scheduler) Acquire(ctx context.Context) (*Lease, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("acquire capacity lease: %w", ctx.Err())
	case <-s.tokens:
		return &Lease{
			release: func() {
				s.tokens <- struct{}{}
			},
		}, nil
	}
}
