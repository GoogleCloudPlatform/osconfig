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

package agentendpoint

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func retrySleep(i int, e int, rnd *rand.Rand) time.Duration {
	// i=1 and e=0 => 1*1+[0,1] => 1-2s
	// i=2 and e=0 => 2*2+[0,2] => 4-6s
	// i=3 and e=0 => 3*3+[0,3] => 9-12s

	// i=1 and e=5 => 6*1+[0,6] => 6-12s
	// i=2 and e=5 => 7*2+[0,7] => 14-21s
	// i=3 and e=5 => 8*3+[0,8] => 24-32s

	// i=1 and e=10 => 11*1+[0,11] => 11-22s
	// i=2 and e=10 => 12*2+[0,12] => 24-36s
	// i=3 and e=10 => 13*3+[0,13] => 39-52s
	nf := math.Min(float64((i+e)*i+rnd.Intn(i+e)), 300)
	return time.Duration(int(nf)) * time.Second
}

func retryFunc(maxRetryTime time.Duration, desc string, f func() error) error {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	var tot time.Duration
	for i := 1; ; i++ {
		err := f()
		if err == nil {
			return nil
		}

		ns := retrySleep(i, 0, rnd)
		tot += ns
		if tot > maxRetryTime {
			return err
		}

		logger.Errorf("Error %s, attempt %d, retrying in %s: %v", desc, i, ns, err)
		time.Sleep(ns)
	}
}

func retryAPICall(maxRetryTime time.Duration, name string, f func() error) error {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
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

		ns := retrySleep(i, extra, rnd)
		tot += ns
		if tot > maxRetryTime {
			return err
		}

		logger.Errorf("Error calling %s, attempt %d, retrying in %s: %v", name, i, ns, err)
		time.Sleep(ns)
	}
}
