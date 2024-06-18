//  Copyright 2017 Google Inc. All Rights Reserved.
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

package packages

import (
	"context"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/golang/mock/gomock"
)

var pkgs = []string{"pkg1", "pkg2"}
var testCtx = context.Background()

type expectedCommand struct {
	cmd    *exec.Cmd
	envs   []string
	stdout []byte
	stderr []byte
	err    error
}

func setExpectations(mockCommandRunner *utilmocks.MockCommandRunner, expectedCommandsChain []expectedCommand) {
	if len(expectedCommandsChain) == 0 {
		return
	}

	var prev *gomock.Call
	for _, expectedCmd := range expectedCommandsChain {
		cmd := expectedCmd.cmd
		if len(expectedCmd.envs) > 0 {
			cmd.Env = append(os.Environ(), expectedCmd.envs...)
		}

		if prev == nil {
			prev = mockCommandRunner.EXPECT().
				Run(testCtx, utilmocks.EqCmd(cmd)).
				Return(expectedCmd.stdout, expectedCmd.stderr, expectedCmd.err).Times(1)
		} else {
			prev = mockCommandRunner.EXPECT().
				Run(testCtx, utilmocks.EqCmd(cmd)).
				After(prev).
				Return(expectedCmd.stdout, expectedCmd.stderr, expectedCmd.err).Times(1)
		}
	}
}

func formatError(err error) string {
	if err == nil {
		return "<nil>"
	}

	return err.Error()
}

func getMockRun(content []byte, err error) func(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
	return func(_ context.Context, cmd *exec.Cmd) ([]byte, error) {
		return content, err
	}
}

// TODO: move this to a common helper package
func helperLoadBytes(name string) ([]byte, error) {
	path := filepath.Join("testdata", name) // relative path
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
