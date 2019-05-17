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
	"os/exec"

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

var patchRunRunner = func(r *patchRun) func(cmd *exec.Cmd) ([]byte, error) {
	return func(cmd *exec.Cmd) ([]byte, error) {
		r.debugf("Running %q with args %q\n", cmd.Path, cmd.Args[1:])
		out, err := cmd.CombinedOutput()
		if err != nil {
			r.debugf("error running %q with args %q: %v, stdout: %s", cmd.Path, cmd.Args, err, out)
			return nil, err
		}
		return out, nil
	}
}
