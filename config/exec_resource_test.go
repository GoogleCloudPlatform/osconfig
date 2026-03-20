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
	"reflect"
	"runtime"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1/agentendpointpb"

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
		wantErr              string
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
			wantErr:              "",
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
			wantErr:              "",
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
			wantErr:              "",
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
			wantErr:              "",
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
			wantErr:              "",
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
			wantErr:              `unsupported interpreter "99"`,
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
			wantErr:              `unrecognized Source type for ExecResource: %!q(<nil>)`,
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
			wantErr:              `unsupported File `,
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
			wantErr:              "",
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
			if !matchError(t, err, tt.wantErr) || tt.wantErr != "" {
				return
			}

			resource := pr.resource.(*execResource)

			if tt.wantValidatePath != "" {
				if tt.wantValidatePath != filepath.Base(resource.validatePath) {
					t.Errorf("unexpected validate path: got %q, want %q", filepath.Base(resource.validatePath), tt.wantValidatePath)
				}
				assertFileContents(t, resource.validatePath, tt.wantValidateContents)
			}

			if tt.wantEnforcePath != "" {
				if tt.wantEnforcePath != filepath.Base(resource.enforcePath) {
					t.Errorf("unexpected enforce path: got %q, want %q", filepath.Base(resource.enforcePath), tt.wantEnforcePath)
				}
				assertFileContents(t, resource.enforcePath, tt.wantEnforceContents)
			}
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
		wantErr     string
	}{
		{
			name:        "NONE interpreter",
			goos:        "linux",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE},
			expectedCmd: exec.Command("test_script"),
			wantErr:     "",
		},
		{
			name:        "SHELL Linux",
			goos:        "linux",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL},
			expectedCmd: exec.Command("/bin/sh", "test_script"),
			wantErr:     "",
		},
		{
			name:        "SHELL with args",
			goos:        "linux",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL, Args: []string{"arg1", "arg2"}},
			expectedCmd: exec.Command("/bin/sh", "test_script", "arg1", "arg2"),
			wantErr:     "",
		},
		{
			name:        "SHELL Windows",
			goos:        "windows",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL},
			expectedCmd: exec.Command("test_script"),
			wantErr:     "",
		},
		{
			name:        "POWERSHELL Windows",
			goos:        "windows",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_POWERSHELL},
			expectedCmd: exec.Command("C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\PowerShell.exe", "-File", "test_script"),
			wantErr:     "",
		},
		{
			name:        "POWERSHELL Linux error",
			goos:        "linux",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_POWERSHELL},
			expectedCmd: nil,
			wantErr:     `interpreter "POWERSHELL" can only be used on Windows systems`,
		},
		{
			name:        "Unsupported interpreter",
			goos:        "linux",
			execR:       &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Interpreter(99)},
			expectedCmd: nil,
			wantErr:     `unsupported interpreter "99"`,
		},
		{
			name:        "Nil Exec",
			goos:        "linux",
			execR:       nil,
			expectedCmd: nil,
			wantErr:     `ExecResource Exec cannot be nil`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goos = tt.goos
			e := &execResource{}
			mockCommandRunner := setupMockRunner(t)
			if tt.expectedCmd != nil {
				mockCommandRunner.EXPECT().Run(ctx, utilmocks.EqCmd(tt.expectedCmd)).Return([]byte("stdout"), []byte("stderr"), nil).Times(1)
			}

			_, _, _, err := e.run(ctx, "test_script", tt.execR)
			matchError(t, err, tt.wantErr)
		})
	}
}

// TestExecResourceCheckState verifies validation phase exit code mapping.
func TestExecResourceCheckState(t *testing.T) {
	ctx := context.Background()
	preserveGlobalState(t)

	var tests = []struct {
		name               string
		exitCode           int
		wantInDesiredState bool
		wantErr            string
	}{
		{name: "Code 100", exitCode: 100, wantInDesiredState: true, wantErr: ""},
		{name: "Code 101", exitCode: 101, wantInDesiredState: false, wantErr: ""},
		{name: "Code 0", exitCode: 0, wantInDesiredState: false, wantErr: "unexpected return code from validate: 0, stdout: stdout, stderr: stderr"},
		{name: "Code -1", exitCode: -1, wantInDesiredState: false, wantErr: "some error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCommandRunner := setupMockRunner(t)
			mockRunnerExpectation(ctx, mockCommandRunner, tt.exitCode)
			e := &execResource{
				validatePath: "test_script",
				OSPolicy_Resource_ExecResource: &agentendpointpb.OSPolicy_Resource_ExecResource{
					Validate: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
						Interpreter: agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
					},
				},
			}

			inDesiredState, retErr := e.checkState(ctx)
			if matchError(t, retErr, tt.wantErr) {
				if inDesiredState != tt.wantInDesiredState {
					t.Errorf("checkState() inDesiredState = %v, want %v", inDesiredState, tt.wantInDesiredState)
				}
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
		wantErr            string
		wantOutput         string
	}{
		{name: "Code 100 without output", exitCode: 100, outputFilePath: "", wantInDesiredState: true, wantErr: "", wantOutput: ""},
		{name: "Code 100 with output", exitCode: 100, outputFilePath: outputFile, wantInDesiredState: true, wantErr: "", wantOutput: "my enforce output"},
		{name: "Code 0", exitCode: 0, outputFilePath: "", wantInDesiredState: false, wantErr: "unexpected return code from enforce: 0, stdout: stdout, stderr: stderr", wantOutput: ""},
		{name: "Code -1", exitCode: -1, outputFilePath: "", wantInDesiredState: false, wantErr: "some error", wantOutput: ""},
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
			e := &execResource{
				enforcePath: "test_script",
				OSPolicy_Resource_ExecResource: &agentendpointpb.OSPolicy_Resource_ExecResource{
					Enforce: &agentendpointpb.OSPolicy_Resource_ExecResource_Exec{
						Interpreter:    agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE,
						OutputFilePath: tt.outputFilePath,
					},
				},
			}

			inDesiredState, retErr := e.enforceState(ctx)
			if matchError(t, retErr, tt.wantErr) {
				if inDesiredState != tt.wantInDesiredState {
					t.Errorf("enforceState() inDesiredState = %v, want %v", inDesiredState, tt.wantInDesiredState)
				}
				if tt.wantOutput != "" && string(e.enforceOutput) != tt.wantOutput {
					t.Errorf("enforceState() output = %q, want %q", string(e.enforceOutput), tt.wantOutput)
				}
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

	_, errDNE := os.Open("DNE")
	expectedDNEErr := fmt.Sprintf("error opening OutputFilePath: %v", errDNE)

	var tests = []struct {
		name     string
		filePath string
		want     []byte
		wantErr  string
	}{
		{
			name:     "empty path",
			filePath: "",
			want:     nil,
			wantErr:  "",
		},
		{
			name:     "path DNE",
			filePath: "DNE",
			want:     nil,
			wantErr:  expectedDNEErr,
		},
		{
			name:     "normal case",
			filePath: fileA,
			want:     contentsA,
			wantErr:  "",
		},
		{
			name:     "file to large case",
			filePath: fileB,
			want:     contentsB[:maxExecOutputSize],
			wantErr:  fmt.Sprintf("contents of OutputFilePath greater than %dK", maxExecOutputSize/1024),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := execOutput(ctx, tt.filePath)
			if matchError(t, err, tt.wantErr) {
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("got != want, string(got) = %q string(want) = %q", got, tt.want)
				}
			}
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
			e := &execResource{
				enforceOutput: tt.outputData,
			}
			rCompliance := &agentendpointpb.OSPolicyResourceCompliance{}
			e.populateOutput(rCompliance)

			var got string
			if rCompliance.GetExecResourceOutput() != nil {
				got = string(rCompliance.GetExecResourceOutput().GetEnforcementOutput())
			}

			if got != tt.wantOutput {
				t.Errorf("populateOutput() output = %q, want %q", got, tt.wantOutput)
			}
		})
	}
}

// TestExecResourceCleanup verifies cleanup of temporary directories.
func TestExecResourceCleanup(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		setupTempDir func() string
		wantErr      string
	}{
		{
			name:         "Empty temp directory",
			setupTempDir: func() string { return "" },
			wantErr:      "",
		},
		{
			name: "Valid temp directory",
			setupTempDir: func() string {
				return t.TempDir()
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := tt.setupTempDir()
			e := &execResource{tempDir: tmpDir}

			err := e.cleanup(ctx)
			matchError(t, err, tt.wantErr)

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

// matchError asserts if the error matches the expected error message.
// It returns true if we should continue testing (i.e. no error occurred and none was expected).
func matchError(t *testing.T, err error, wantErr string) bool {
	t.Helper()
	if err != nil {
		if wantErr == "" {
			t.Errorf("Unexpected error: %v", err)
		} else if err.Error() != wantErr {
			t.Errorf("error = %q, wantErr %q", err.Error(), wantErr)
		}
		return false
	}
	if wantErr != "" {
		t.Errorf("Expected error %q but got nil", wantErr)
		return false
	}
	return true
}

// assertFileContents verifies that the file at filePath matches the expected contents.
func assertFileContents(t *testing.T, filePath string, wantContents string) {
	t.Helper()
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file %q: %v", filePath, err)
	}
	if string(data) != wantContents {
		t.Errorf("File contents = %q, want %q", string(data), wantContents)
	}
}
