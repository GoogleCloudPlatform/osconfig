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

package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestSchedulerAcquireAndRelease(t *testing.T) {
	s, err := New(2)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx := context.Background()
	lease1, err := s.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire(1) error: %v", err)
	}

	lease2, err := s.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire(2) error: %v", err)
	}

	// Channel is now empty. Acquiring with a short timeout should fail.
	timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	if _, err := s.Acquire(timeoutCtx); err == nil {
		t.Fatal("expected timeout when acquiring with exhausted capacity")
	}

	// Release one lease and acquire again.
	lease1.Release()
	// Idempotent release check
	lease1.Release()

	lease3, err := s.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire(3) after release error: %v", err)
	}

	lease2.Release()
	lease3.Release()
}

func TestSchedulerInvalidCapacity(t *testing.T) {
	if _, err := New(0); err == nil {
		t.Fatal("expected error for capacity 0")
	}
	if _, err := New(-1); err == nil {
		t.Fatal("expected error for negative capacity")
	}
}
