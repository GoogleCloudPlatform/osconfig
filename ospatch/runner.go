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

package ospatch

import (
	"math"
	"math/rand"
	"os/exec"
	"time"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
)

var defaultRunner = func(cmd *exec.Cmd) ([]byte, error) {
	logger.Debugf("Running %q with args %q\n", cmd.Path, cmd.Args[1:])
	out, err := cmd.CombinedOutput()
	if err != nil {
		logger.Debugf("error running %q with args %q: %v, stdout: %s", cmd.Path, cmd.Args, err, out)
		return nil, err
	}
	return out, nil
}

// retry tries to retry f for no more than maxRetryTime.
func retry2(maxRetryTime time.Duration, desc string, logF func(string, ...interface{}), f func() error) error {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	var tot time.Duration
	for i := 1; ; i++ {
		err := f()
		if err == nil {
			return nil
		}

		// Always increasing with some jitter, longest wait will be 5min.
		nf := math.Min(float64(i)*float64(i)+float64(rnd.Intn(i)), 300)
		ns := time.Duration(int(nf)) * time.Second
		tot += ns
		if tot > maxRetryTime {
			return err
		}

		logF("Error %s, attempt %d, retrying in %s: %v", desc, i, ns, err)
		time.Sleep(ns)
	}
}
