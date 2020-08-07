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

// Package retryutil provides utility functions for retrying.
package retryutil

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RetrySleep returns a pseudo-random sleep duration.
func RetrySleep(base int, extra int) time.Duration {
	// base=1 and extra=0 => 1*1+[0,1] => 1-2s
	// base=2 and extra=0 => 2*2+[0,2] => 4-6s
	// base=3 and extra=0 => 3*3+[0,3] => 9-12s

	// base=1 and extra=5 => 6*1+[0,6] => 6-12s
	// base=2 and extra=5 => 7*2+[0,7] => 14-21s
	// base=3 and extra=5 => 8*3+[0,8] => 24-32s

	// base=1 and extra=10 => 11*1+[0,11] => 11-22s
	// base=2 and extra=10 => 12*2+[0,12] => 24-36s
	// base=3 and extra=10 => 13*3+[0,13] => 39-52s
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	nf := math.Min(float64((base+extra)*base+rnd.Intn(base+extra)), 300)
	return time.Duration(int(nf)) * time.Second
}

// RetryFunc retries a function provided as a parameter for maxRetryTime.
func RetryFunc(ctx context.Context, maxRetryTime time.Duration, desc string, f func() error) error {
	var tot time.Duration
	for i := 1; ; i++ {
		err := f()
		if err == nil {
			return nil
		}

		ns := RetrySleep(i, 0)
		tot += ns
		if tot > maxRetryTime {
			return err
		}

		clog.Errorf(ctx, "Error %s, attempt %d, retrying in %s: %v", desc, i, ns, err)
		time.Sleep(ns)
	}
}

// RetryAPICall retries an API call for maxRetryTime.
func RetryAPICall(ctx context.Context, maxRetryTime time.Duration, name string, f func() error) error {
	var tot time.Duration
	for i := 1; ; i++ {
		extra := 1
		err := f()
		if err == nil {
			return nil
		}
		if s, ok := status.FromError(err); ok {
			err := fmt.Errorf("code: %q, message: %q, details: %q", s.Code(), s.Message(), s.Details())
			switch s.Code() {
			// Errors we should retry.
			case codes.DeadlineExceeded, codes.Unavailable, codes.Aborted, codes.Internal:
			// Add additional sleep.
			case codes.ResourceExhausted:
				extra = 10
			default:
				return err
			}
		} else {
			return err
		}

		ns := RetrySleep(i, extra)
		tot += ns
		if tot > maxRetryTime {
			return err
		}

		clog.Warningf(ctx, "Error calling %s, attempt %d, retrying in %s: %v", name, i, ns, err)
		time.Sleep(ns)
	}
}
