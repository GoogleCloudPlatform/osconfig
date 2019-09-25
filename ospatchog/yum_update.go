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

package ospatchog

import (
	"os/exec"
)

const yum = "/usr/bin/yum"

var (
	yumUpdateArgs        = []string{"update", "-y"}
	yumUpdateMinimalArgs = []string{"update-minimal", "-y"}
)

type yumUpdateOpts struct {
	security bool
	minimal  bool
	excludes []string
	runner   func(cmd *exec.Cmd) ([]byte, error)
}

// YumUpdateOption is an option for yum update.
type YumUpdateOption func(*yumUpdateOpts)

// YumUpdateSecurity returns a YumUpdateOption that specifies the --security flag should
// be used.
func YumUpdateSecurity(security bool) YumUpdateOption {
	return func(args *yumUpdateOpts) {
		args.security = security
	}
}

// YumUpdateMinimal returns a YumUpdateOption that specifies the update-minimal
// command should be used.
func YumUpdateMinimal(minimal bool) YumUpdateOption {
	return func(args *yumUpdateOpts) {
		args.minimal = minimal
	}
}

// YumUpdateExcludes returns a YumUpdateOption that specifies what packages to add to
// the --exclude flag.
func YumUpdateExcludes(excludes []string) YumUpdateOption {
	return func(args *yumUpdateOpts) {
		args.excludes = excludes
	}
}

// YumUpdateRunner returns a YumUpdateOption that specifies the runner.
func YumUpdateRunner(runner func(cmd *exec.Cmd) ([]byte, error)) YumUpdateOption {
	return func(args *yumUpdateOpts) {
		args.runner = runner
	}
}

// RunYumUpdate runs yum update.
func RunYumUpdate(opts ...YumUpdateOption) error {
	yumOpts := &yumUpdateOpts{
		security: false,
		minimal:  false,
		excludes: nil,
		runner:   defaultRunner,
	}

	for _, opt := range opts {
		opt(yumOpts)
	}

	args := yumUpdateArgs
	if yumOpts.minimal {
		args = yumUpdateMinimalArgs
	}
	if yumOpts.security {
		args = append(args, "--security")
	}
	for _, e := range yumOpts.excludes {
		args = append(args, "--exclude="+e)
	}

	if _, err := yumOpts.runner(exec.Command(yum, args...)); err != nil {
		return err
	}
	return nil
}
