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

package gcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const observationLimit = 5

// Check returns a human-readable observation, whether the desired state was
// reached, and a permanent error. Transient states belong in observation.
type Check func(context.Context) (observation string, done bool, err error)

// WaitError explains a cancellation with the most recent observable state.
type WaitError struct {
	Description  string
	Elapsed      time.Duration
	Attempts     int
	Observations []string
	Cause        error
}

func (e *WaitError) Error() string {
	last := "none"
	if len(e.Observations) > 0 {
		last = strings.Join(e.Observations, " | ")
	}
	return fmt.Sprintf("wait for %s stopped after %s (%d attempts): %v; recent observations: %s", e.Description, e.Elapsed.Round(time.Millisecond), e.Attempts, e.Cause, last)
}

func (e *WaitError) Unwrap() error { return e.Cause }

// PollUntil checks immediately, then polls with a stoppable timer until success,
// a permanent error, or context cancellation.
func PollUntil(ctx context.Context, interval time.Duration, description string, check Check) error {
	if interval <= 0 {
		return fmt.Errorf("poll interval must be positive")
	}
	start := time.Now()
	var observations []string
	for attempts := 1; ; attempts++ {
		observation, done, err := check(ctx)
		if observation != "" {
			observations = append(observations, observation)
			if len(observations) > observationLimit {
				observations = observations[len(observations)-observationLimit:]
			}
		}
		if err != nil {
			return fmt.Errorf("wait for %s: %w", description, err)
		}
		if done {
			return nil
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			cause := context.Cause(ctx)
			if cause == nil {
				cause = errors.New("context canceled")
			}
			return &WaitError{Description: description, Elapsed: time.Since(start), Attempts: attempts, Observations: observations, Cause: cause}
		case <-timer.C:
		}
	}
}
