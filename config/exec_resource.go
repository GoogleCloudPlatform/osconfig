//  Copyright 2020 Google Inc. All Rights Reserved.
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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/util"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
)

const maxExecOutputSize = 100 * 1024

var runner = util.CommandRunner(&util.DefaultRunner{})

type execResource struct {
	*agentendpointpb.OSPolicy_Resource_ExecResource

	validatePath, enforcePath, tempDir string
	enforceOutput                      []byte
}

// TODO: use a persistent cache for downloaded files so we dont need to redownload them each time
func (e *execResource) download(ctx context.Context, execR *agentendpointpb.OSPolicy_Resource_ExecResource_Exec) (string, error) {
	tmpDir, err := ioutil.TempDir(e.tempDir, "")
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %s", err)
	}

	// File extensions are important on Windows.
	var name string
	perms := os.FileMode(0644)
	switch execR.GetSource().(type) {
	case *agentendpointpb.OSPolicy_Resource_ExecResource_Exec_Script:
		switch execR.GetInterpreter() {
		case agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE:
			if goos == "windows" {
				name = "script.cmd"
			} else {
				name = "script"
			}
			perms = os.FileMode(0755)
		case agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL:
			if goos == "windows" {
				name = "script.cmd"
			} else {
				name = "script.sh"
			}
		case agentendpointpb.OSPolicy_Resource_ExecResource_Exec_POWERSHELL:
			name = "script.ps1"
		default:
			return "", fmt.Errorf("unsupported interpreter %q", execR.GetInterpreter())
		}
		name = filepath.Join(tmpDir, name)
		if _, err := util.AtomicWriteFileStream(strings.NewReader(execR.GetScript()), "", name, perms); err != nil {
			return "", err
		}

	case *agentendpointpb.OSPolicy_Resource_ExecResource_Exec_File:
		if execR.GetFile().GetLocalPath() != "" {
			return execR.GetFile().GetLocalPath(), nil
		}
		switch {
		case execR.GetFile().GetGcs().GetObject() != "":
			name = path.Base(execR.GetFile().GetGcs().GetObject())
		case execR.GetFile().GetRemote().GetUri() != "":
			name = path.Base(execR.GetFile().GetRemote().GetUri())
		default:
			return "", fmt.Errorf("unsupported File %v", execR.GetFile())
		}
		if execR.GetInterpreter() == agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE {
			perms = os.FileMode(0755)
		}
		name = filepath.Join(tmpDir, name)
		if _, err := downloadFile(ctx, name, perms, execR.GetFile()); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unrecognized Source type for ExecResource: %q", execR.GetSource())
	}

	return name, nil
}

func (e *execResource) validate(ctx context.Context) (*ManagedResources, error) {
	tmpDir, err := ioutil.TempDir("", "osconfig_exec_resource_")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %s", err)
	}
	e.tempDir = tmpDir

	e.validatePath, err = e.download(ctx, e.GetValidate())
	if err != nil {
		return nil, err
	}

	// Assume lack of Enforce means policy is in VALIDATE mode.
	if e.GetEnforce() != nil {
		e.enforcePath, err = e.download(ctx, e.GetEnforce())
		if err != nil {
			return nil, err
		}
	}

	return nil, nil
}

func (e *execResource) run(ctx context.Context, name string, execR *agentendpointpb.OSPolicy_Resource_ExecResource_Exec) ([]byte, []byte, int, error) {
	if execR == nil {
		return nil, nil, 0, fmt.Errorf("ExecResource Exec cannot be nil")
	}

	var cmd string
	var args []string
	switch execR.GetInterpreter() {
	case agentendpointpb.OSPolicy_Resource_ExecResource_Exec_NONE:
		cmd = name
	case agentendpointpb.OSPolicy_Resource_ExecResource_Exec_SHELL:
		if goos == "windows" {
			cmd = name
		} else {
			args = append([]string{name})
			cmd = "/bin/sh"
		}
	case agentendpointpb.OSPolicy_Resource_ExecResource_Exec_POWERSHELL:
		if goos != "windows" {
			return nil, nil, 0, fmt.Errorf("interpreter %q can only be used on Windows systems", execR.GetInterpreter())
		}
		args = append([]string{"-File", name})
		cmd = "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\PowerShell.exe"
	default:
		return nil, nil, 0, fmt.Errorf("unsupported interpreter %q", execR.GetInterpreter())
	}
	args = append(args, execR.GetArgs()...)

	stdout, stderr, err := runner.Run(ctx, exec.Command(cmd, args...))
	code := 0
	if err != nil {
		code = -1
		if v, ok := err.(*exec.ExitError); ok {
			code = v.ExitCode()
		}
	}
	return stdout, stderr, code, err
}

func (e *execResource) checkState(ctx context.Context) (inDesiredState bool, err error) {
	// For validate we expect an exit code of 100 for "correct state" and 101 for "incorrect state".
	// 100 was chosen over 0 (and 101 vs 1) because we want an explicit indicator of
	// "correct" vs "incorrect" state and errors. Also Powershell will always exit 0 unless "exit"
	// is explicitly called.
	// A code of -1 indicates some other error, so we just return err.
	stdout, stderr, code, err := e.run(ctx, e.validatePath, e.GetValidate())
	switch code {
	case -1:
		return false, err
	case 100:
		return true, nil
	case 101:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected return code from validate: %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
}

func (e *execResource) enforceState(ctx context.Context) (inDesiredState bool, err error) {
	// For enforce we expect an exit code of 100 for "success" and anything positive code is a failure".
	// 100 was chosen over 0 because we want an explicit indicator of "sucess" vs errors.
	// Also Powershell will always exit 0 unless "exit" is explicitly called.
	// A code of -1 indicates some other error, so we just return err.
	stdout, stderr, code, err := e.run(ctx, e.enforcePath, e.GetEnforce())
	switch code {
	case -1:
		return false, err
	case 100:
		out, err := execOutput(ctx, e.GetEnforce().GetOutputFilePath())
		e.enforceOutput = out
		return true, err
	default:
		return false, fmt.Errorf("unexpected return code from enforce: %d, stdout: %s, stderr: %s", code, stdout, stderr)
	}
}

func execOutput(ctx context.Context, outputFilePath string) ([]byte, error) {
	if outputFilePath == "" {
		return nil, nil
	}

	clog.Debugf(ctx, "Reading %q for ExecResource output", outputFilePath)

	f, err := os.Open(outputFilePath)
	if err != nil {
		return nil, fmt.Errorf("error opening OutputFilePath: %v", err)
	}
	defer f.Close()

	// Make a byte slice with a capacity of 1 over maxSize (for checking maxExecOutputSize).
	output := make([]byte, 0, maxExecOutputSize+1)
	// Read up to capactity.
	n, err := f.Read(output[:cap(output)])
	output = output[:len(output)+n]
	if err != nil && err != io.EOF {
		return output, fmt.Errorf("error reading from OutputFilePath: %v", err)
	}
	// Return the output up to this point and an error if total size is greater than maxExecOutputSize.
	if n > maxExecOutputSize {
		return output[:maxExecOutputSize], fmt.Errorf("contents of OutputFilePath greater than %dK", maxExecOutputSize/1024)
	}
	return output, nil
}

func (e *execResource) populateOutput(rCompliance *agentendpointpb.OSPolicyResourceCompliance) {
	if e.enforceOutput != nil {
		rCompliance.Output = &agentendpointpb.OSPolicyResourceCompliance_ExecResourceOutput_{
			ExecResourceOutput: &agentendpointpb.OSPolicyResourceCompliance_ExecResourceOutput{
				EnforcementOutput: e.enforceOutput,
			},
		}
	}
}

func (e *execResource) cleanup(ctx context.Context) error {
	if e.tempDir != "" {
		return os.RemoveAll(e.tempDir)
	}
	return nil
}
