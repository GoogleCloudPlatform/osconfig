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
	"os"
	"os/exec"
	"path/filepath"
)

var (
	googet = filepath.Join(os.Getenv("GooGetRoot"), "googet.exe")

	googetUpdateArgs = []string{"-noconfirm", "update"}
)

type googetUpdateOpts struct {
	runner func(cmd *exec.Cmd) ([]byte, error)
}

// GooGetUpdateOption is an option for apt-get update.
type GooGetUpdateOption func(*googetUpdateOpts)

// GooGetUpdateRunner returns a GooGetUpdateOption that specifies the runner.
func GooGetUpdateRunner(runner func(cmd *exec.Cmd) ([]byte, error)) GooGetUpdateOption {
	return func(args *googetUpdateOpts) {
		args.runner = runner
	}
}

// RunGooGetUpdate runs googet update.
func RunGooGetUpdate(opts ...GooGetUpdateOption) error {
	googetOpts := &googetUpdateOpts{
		runner: defaultRunner,
	}

	for _, opt := range opts {
		opt(googetOpts)
	}

	if _, err := googetOpts.runner(exec.Command(googet, googetUpdateArgs...)); err != nil {
		return err
	}
	return nil
}
