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
)

const zypper = "/usr/bin/zypper"

var (
	zypperUpdateArgs = []string{"update"}
)

type zypperUpdateOpts struct {
	runner func(cmd *exec.Cmd) ([]byte, error)
}

// ZypperUpdateOption is an option for zypper update.
type ZypperUpdateOption func(*zypperUpdateOpts)

// ZypperUpdateRunner returns a ZypperUpdateOption that specifies the runner.
func ZypperUpdateRunner(runner func(cmd *exec.Cmd) ([]byte, error)) ZypperUpdateOption {
	return func(args *zypperUpdateOpts) {
		args.runner = runner
	}
}

// RunZypperUpdate runs zypper update.
func RunZypperUpdate(opts ...ZypperUpdateOption) error {
	zOpts := &zypperUpdateOpts{
		runner: defaultRunner,
	}

	for _, opt := range opts {
		opt(zOpts)
	}

	if _, err := zOpts.runner(exec.Command(zypper, zypperUpdateArgs...)); err != nil {
		return err
	}
	return nil
}
