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
	zypperPatchArgs = []string{"patch", "-y"}
)

type zypperPatchOpts struct {
	categories   []string
	severities   []string
	withOptional bool
	withUpdate   bool
	runner       func(cmd *exec.Cmd) ([]byte, error)
}

// ZypperPatchOption is an option for zypper patch.
type ZypperPatchOption func(*zypperPatchOpts)

// ZypperPatchCategories returns a ZypperUpdateOption that specifies what
// categories to add to the --categories flag.
func ZypperPatchCategories(categories []string) ZypperPatchOption {
	return func(args *zypperPatchOpts) {
		args.categories = categories
	}
}

// ZypperPatchSeverities returns a ZypperUpdateOption that specifies what
// categories to add to the --categories flag.
func ZypperPatchSeverities(severities []string) ZypperPatchOption {
	return func(args *zypperPatchOpts) {
		args.severities = severities
	}
}

// ZypperUpdateWithOptional returns a ZypperUpdateOption that specifies the
// --with-optional flag should be used.
func ZypperUpdateWithOptional(withOptional bool) ZypperPatchOption {
	return func(args *zypperPatchOpts) {
		args.withOptional = withOptional
	}
}

// ZypperUpdateWithUpdate returns a ZypperUpdateOption that specifies the
// --with-update flag should be used.
func ZypperUpdateWithUpdate(withUpdate bool) ZypperPatchOption {
	return func(args *zypperPatchOpts) {
		args.withUpdate = withUpdate
	}
}

// ZypperPatchRunner returns a ZypperUpdateOption that specifies the runner.
func ZypperPatchRunner(runner func(cmd *exec.Cmd) ([]byte, error)) ZypperPatchOption {
	return func(args *zypperPatchOpts) {
		args.runner = runner
	}
}

// RunZypperPatch runs zypper patch.
func RunZypperPatch(opts ...ZypperPatchOption) error {
	zOpts := &zypperPatchOpts{
		runner: defaultRunner,
	}

	for _, opt := range opts {
		opt(zOpts)
	}

	args := zypperPatchArgs
	if zOpts.withOptional {
		args = append(args, "--with-optional")
	}
	if zOpts.withUpdate {
		args = append(args, "--with-update")
	}
	for _, c := range zOpts.categories {
		args = append(args, "--category="+c)
	}
	for _, s := range zOpts.severities {
		args = append(args, "--severity="+s)
	}

	if _, err := zOpts.runner(exec.Command(zypper, args...)); err != nil {
		return err
	}
	return nil
}
