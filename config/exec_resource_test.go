//  Copyright 2021 Google Inc. All Rights Reserved.
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

package config

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"

	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/golang/mock/gomock"
)

// TestExecResourceDownload verifies downloading and temp file creation for exec resources.
func TestExecResourceDownload(t *testing.T) {
	ctx := context.Background()
	preserveGlobalState(t)

	tmpDir := t.TempDir()
	localScriptPath := filepath.Join(tmpDir, "my_local_script")
	if err := os.WriteFile(localScriptPath, []byte("local validate"), 0755); err != nil {
		t.Fatal(err)
	}

	var tests = []struct {
		name                 string
		erpb                 *agentendpointpb.OSPolicy_Resource_ExecResource
		wantValidatePath     string
		wantValidateContents string
		wantEnforcePath      string
		wantEnforceContents  string
		goos                 string
		wantErr              error
	}{
		{
			name: "Script NONE Linux",
			erpb: &agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "validate"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
				},
				Enforce: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "enforce"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
				},
			},
			wantValidatePath:     "script",
			wantValidateContents: "validate",
			wantEnforcePath:      "script",
			wantEnforceContents:  "enforce",
			goos:                 "linux",
			wantErr:              nil,
		},
		{
			name: "Script NONE Windows",
			erpb: &agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "validate"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
				},
				Enforce: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "enforce"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
				},
			},
			wantValidatePath:     "script.cmd",
			wantValidateContents: "validate",
			wantEnforcePath:      "script.cmd",
			wantEnforceContents:  "enforce",
			goos:                 "windows",
			wantErr:              nil,
		},
		{
			name: "Script SHELL Linux",
			erpb: &agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "validate"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL,
				},
				Enforce: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "enforce"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL,
				},
			},
			wantValidatePath:     "script.sh",
			wantValidateContents: "validate",
			wantEnforcePath:      "script.sh",
			wantEnforceContents:  "enforce",
			goos:                 "linux",
			wantErr:              nil,
		},
		{
			name: "Script SHELL Windows",
			erpb: &agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "validate"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL,
				},
				Enforce: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "enforce"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL,
				},
			},
			wantValidatePath:     "script.cmd",
			wantValidateContents: "validate",
			wantEnforcePath:      "script.cmd",
			wantEnforceContents:  "enforce",
			goos:                 "windows",
			wantErr:              nil,
		},
		{
			name: "Script POWERSHELL Windows",
			erpb: &agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "validate"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_POWERSHELL,
				},
				Enforce: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "enforce"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_POWERSHELL,
				},
			},
			wantValidatePath:     "script.ps1",
			wantValidateContents: "validate",
			wantEnforcePath:      "script.ps1",
			wantEnforceContents:  "enforce",
			goos:                 "windows",
			wantErr:              nil,
		},
		{
			name: "Unsupported Interpreter",
			erpb: &agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source:      &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script{Script: "validate"},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Interpreter(99),
				},
			},
			wantValidatePath:     "",
			wantValidateContents: "",
			wantEnforcePath:      "",
			wantEnforceContents:  "",
			goos:                 "linux",
			wantErr:              errors.New(`unsupported interpreter "99"`),
		},
		{
			name: "Unrecognized Source Type",
			erpb: &agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
				},
			},
			wantValidatePath:     "",
			wantValidateContents: "",
			wantEnforcePath:      "",
			wantEnforceContents:  "",
			goos:                 "linux",
			wantErr:              errors.New(`unrecognized Source type for ExecResource: %!q(<nil>)`),
		},
		{
			name: "Unsupported File",
			erpb: &agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_File{
						File: &agentendpointpb.OSPolicy_Resource_File{},
					},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
				},
			},
			wantValidatePath:     "",
			wantValidateContents: "",
			wantEnforcePath:      "",
			wantEnforceContents:  "",
			goos:                 "linux",
			wantErr:              errors.New(`unsupported File `),
		},
		{
			name: "LocalPath File",
			erpb: &agentendpointpb.OSPolicy_Resource_ExecResource{
				Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
					Source: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec_File{
						File: &agentendpointpb.OSPolicy_Resource_File{
							Type: &agentendpointpb.OSPolicy_Resource_File_LocalPath{LocalPath: localScriptPath},
						},
					},
					Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
				},
			},
			wantValidatePath:     "my_local_script",
			wantValidateContents: "local validate",
			wantEnforcePath:      "",
			wantEnforceContents:  "",
			goos:                 "linux",
			wantErr:              nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goos = tt.goos
			pr := &OSPolicyResource{
				OSPolicy_Resource: &agentendpointpb.OSPolicy_Resource{
					ResourceType: &agentendpointpb.OSPolicy_Resource_Exec{
						Exec: tt.erpb,
					},
				},
			}
			defer pr.Cleanup(ctx)

			err := pr.Validate(ctx)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)

			resource := pr.resource.(*execResource)

			utiltest.AssertFilePath(t, resource.validatePath, tt.wantValidatePath)
			utiltest.AssertFileContents(t, resource.validatePath, tt.wantValidateContents)

			utiltest.AssertFilePath(t, resource.enforcePath, tt.wantEnforcePath)
			utiltest.AssertFileContents(t, resource.enforcePath, tt.wantEnforceContents)
		})
	}
}

// TestExecResourceRun verifies command construction and execution.
func TestExecResourceRun(t *testing.T) {
	ctx := context.Background()
	preserveGlobalState(t)

	var tests = []struct {
		name        string
		goos        string
		execR       *agentendpointpb.OSPolicy_Resource_ExecResource_Exec
		expectedCmd *exec.Cmd
		wantErr     error
	}{
		{
			name:        "NONE interpreter",
			goos:        "linux",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE},
			expectedCmd: exec.Command("test_script"),
			wantErr:     nil,
		},
		{
			name:        "SHELL Linux",
			goos:        "linux",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL},
			expectedCmd: exec.Command("/bin/sh", "test_script"),
			wantErr:     nil,
		},
		{
			name:        "SHELL with args",
			goos:        "linux",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL, Args: []string{"arg1", "arg2"}},
			expectedCmd: exec.Command("/bin/sh", "test_script", "arg1", "arg2"),
			wantErr:     nil,
		},
		{
			name:        "SHELL Windows",
			goos:        "windows",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL},
			expectedCmd: exec.Command("test_script"),
			wantErr:     nil,
		},
		{
			name:        "POWERSHELL Windows",
			goos:        "windows",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_POWERSHELL},
			expectedCmd: exec.Command("C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\PowerShell.exe", "-File", "test_script"),
			wantErr:     nil,
		},
		{
			name:        "POWERSHELL Linux error",
			goos:        "linux",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_POWERSHELL},
			expectedCmd: nil,
			wantErr:     errors.New(`interpreter "POWERSHELL" can only be used on Windows systems`),
		},
		{
			name:        "Unsupported interpreter",
			goos:        "linux",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Interpreter(99)},
			expectedCmd: nil,
			wantErr:     errors.New(`unsupported interpreter "99"`),
		},
		{
			name:        "Nil Exec",
			goos:        "linux",
			execR:       nil,
			expectedCmd: nil,
			wantErr:     errors.New(`ExecResource Exec cannot be nil`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goos = tt.goos
			execRes := &execResource{}
			mockCommandRunner := setupMockRunner(t)
			if tt.expectedCmd != nil {
				mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(tt.expectedCmd)).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
			}

			_, _, _, err := execRes.run(ctx, "test_script", tt.execR)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}

// TestExecResourceCheckState verifies validation phase exit code mapping.
func TestExecResourceCheckState(t *testing.T) {
	ctx := context.Background()
	preserveGlobalState(t)
	execRes := &execResource{
		validatePath: "test_script",
		OSPolicy_Resource_ExecResource: &agentendpointpb.OSPolicy_Resource_ExecResource{
			Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
				Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
			},
		},
	}

	var tests = []struct {
		name               string
		exitCode           int
		wantInDesiredState bool
		wantErr            error
	}{
		{name: "Code 100", exitCode: 100, wantInDesiredState: true, wantErr: nil},
		{name: "Code 101", exitCode: 101, wantInDesiredState: false, wantErr: nil},
		{name: "Code 0", exitCode: 0, wantInDesiredState: false, wantErr: errors.New("unexpected return code from validate: 0, stdout: stdout, stderr: stderr")},
		{name: "Code -1", exitCode: -1, wantInDesiredState: false, wantErr: errors.New("some error")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCommandRunner := setupMockRunner(t)
			mockRunnerExpectation(ctx, mockCommandRunner, tt.exitCode)

			inDesiredState, err := execRes.checkState(ctx)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
			if inDesiredState != tt.wantInDesiredState {
				t.Errorf("checkState() inDesiredState = %v, want %v", inDesiredState, tt.wantInDesiredState)
			}
		})
	}
}

// TestExecResourceEnforceState verifies enforcement execution and output capturing.
func TestExecResourceEnforceState(t *testing.T) {
	ctx := context.Background()
	preserveGlobalState(t)

	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	var tests = []struct {
		name               string
		exitCode           int
		outputFilePath     string
		wantInDesiredState bool
		wantErr            error
		wantOutput         string
	}{
		{name: "Code 100 without output", exitCode: 100, outputFilePath: "", wantInDesiredState: true, wantErr: nil, wantOutput: ""},
		{name: "Code 100 with output", exitCode: 100, outputFilePath: outputFile, wantInDesiredState: true, wantErr: nil, wantOutput: "my enforce output"},
		{name: "Code 0", exitCode: 0, outputFilePath: "", wantInDesiredState: false, wantErr: errors.New("unexpected return code from enforce: 0, stdout: stdout, stderr: stderr"), wantOutput: ""},
		{name: "Code -1", exitCode: -1, outputFilePath: "", wantInDesiredState: false, wantErr: errors.New("some error"), wantOutput: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCommandRunner := setupMockRunner(t)
			mockRunnerExpectation(ctx, mockCommandRunner, tt.exitCode)
			if tt.outputFilePath != "" {
				if err := os.WriteFile(tt.outputFilePath, []byte(tt.wantOutput), 0644); err != nil {
					t.Fatal(err)
				}
			}
			execRes := &execResource{
				enforcePath: "test_script",
				OSPolicy_Resource_ExecResource: &agentendpointpb.OSPolicy_Resource_ExecResource{
					Enforce: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
						Interpreter:    agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
						OutputFilePath: tt.outputFilePath,
					},
				},
			}

			inDesiredState, err := execRes.enforceState(ctx)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
			if inDesiredState != tt.wantInDesiredState {
				t.Errorf("enforceState() inDesiredState = %v, want %v", inDesiredState, tt.wantInDesiredState)
			}
			if string(execRes.enforceOutput) != tt.wantOutput {
				t.Errorf("enforceState() output = %q, want %q", string(execRes.enforceOutput), tt.wantOutput)
			}
		})
	}
}

// TestExecOutput verifies file reading and truncation for enforce output.
func TestExecOutput(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	fileA := filepath.Join(tmpDir, "fileA")
	contentsA := []byte("here is some text\nand some more\n")
	if err := os.WriteFile(fileA, contentsA, 0600); err != nil {
		t.Fatal(err)
	}

	fileB := filepath.Join(tmpDir, "fileB")
	contentsB := make([]byte, maxExecOutputSize*2)
	if _, err := rand.Read(contentsB); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, contentsB, 0600); err != nil {
		t.Fatal(err)
	}

	_, err := os.Open("DNE")
	wantedDoNotExistErrorMessage := fmt.Sprintf("error opening OutputFilePath: %v", err)

	var tests = []struct {
		name     string
		filePath string
		want     []byte
		wantErr  error
	}{
		{
			name:     "empty path",
			filePath: "",
			want:     nil,
			wantErr:  nil,
		},
		{
			name:     "path DNE",
			filePath: "DNE",
			want:     nil,
			wantErr:  errors.New(wantedDoNotExistErrorMessage),
		},
		{
			name:     "normal case",
			filePath: fileA,
			want:     contentsA,
			wantErr:  nil,
		},
		{
			name:     "file to large case",
			filePath: fileB,
			want:     contentsB[:maxExecOutputSize],
			wantErr:  fmt.Errorf("contents of OutputFilePath greater than %dK", maxExecOutputSize/1024),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := execOutput(ctx, tt.filePath)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
			utiltest.AssertEquals(t, got, tt.want)
		})
	}
}

// TestExecResourcePopulateOutput verifies protobuf output assignment.
func TestExecResourcePopulateOutput(t *testing.T) {
	tests := []struct {
		name       string
		outputData []byte
		wantOutput string
	}{
		{
			name:       "With output data",
			outputData: []byte("test output data"),
			wantOutput: "test output data",
		},
		{
			name:       "Nil output data",
			outputData: nil,
			wantOutput: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execRes := &execResource{
				enforceOutput: tt.outputData,
			}
			rCompliance := &agentendpointpb.OSPolicyResourceCompliance{}
			execRes.populateOutput(rCompliance)

			var got string
			if rCompliance.GetExecResourceOutput() != nil {
				got = string(rCompliance.GetExecResourceOutput().GetEnforcementOutput())
			}
			utiltest.AssertEquals(t, got, tt.wantOutput)
		})
	}
}

// TestExecResourceCleanup verifies cleanup of temporary directories.
func TestExecResourceCleanup(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		setupTempDir func() string
		wantErr      error
	}{
		{
			name:         "Empty temp directory",
			setupTempDir: func() string { return "" },
			wantErr:      nil,
		},
		{
			name: "Valid temp directory",
			setupTempDir: func() string {
				return t.TempDir()
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := tt.setupTempDir()
			execRes := &execResource{tempDir: tmpDir}

			err := execRes.cleanup(ctx)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
			if tmpDir != "" {
				if _, err := os.Stat(tmpDir); !os.IsNotExist(err) {
					t.Errorf("cleanup() failed to remove temp directory %q", tmpDir)
				}
			}
		})
	}
}

// mockRunnerExpectation configures a mock command runner for a single execution.
func mockRunnerExpectation(ctx context.Context, mockCommandRunner *utilmocks.MockCommandRunner, exitCode int) {
	var err error
	if exitCode == -1 {
		err = errors.New("some error")
	} else if exitCode != 0 {
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", fmt.Sprintf("exit %d", exitCode))
		} else {
			cmd = exec.Command("sh", "-c", fmt.Sprintf("exit %d", exitCode))
		}
		err = cmd.Run()
	}
	mockCommandRunner.EXPECT().Run(ctx, gomock.Any()).Return([]byte("stdout"), []byte("stderr"), err).Times(1)
}

// setupMockRunner initializes a gomock controller, creates a mock command runner,
// and injects it into the global runner variable. It also registers a cleanup
// function to finish the mock controller when the test ends.
func setupMockRunner(t *testing.T) *utilmocks.MockCommandRunner {
	t.Helper()
	mockCtrl := gomock.NewController(t)
	t.Cleanup(func() { mockCtrl.Finish() })

	mockCommandRunner := utilmocks.NewMockCommandRunner(mockCtrl)
	runner = mockCommandRunner
	return mockCommandRunner
}

// preserveGlobalState saves the current values of global variables (goos, runner)
// and registers a cleanup function to restore them after the test completes,
// preventing state pollution between tests.
func preserveGlobalState(t *testing.T) {
	t.Helper()
	origGoos := goos
	origRunner := runner

	t.Cleanup(func() {
		goos = origGoos
		runner = origRunner
	})
}
