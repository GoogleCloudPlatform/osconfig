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

package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

var extensionMap = map[agentendpointpb.SoftwareRecipe_Step_RunScript_Interpreter]string{
	agentendpointpb.SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED: ".bat",
	agentendpointpb.SoftwareRecipe_Step_RunScript_SHELL:                   ".bat",
	agentendpointpb.SoftwareRecipe_Step_RunScript_POWERSHELL:              ".ps1",
}

func execFile(step *agentendpointpb.SoftwareRecipe_Step_ExecFile, artifacts map[string]string, runEnvs []string, stepDir string) error {
	var path string
	switch {
	case step.GetArtifactId() != "":
		var ok bool
		artifact := step.GetArtifactId()
		path, ok = artifacts[artifact]
		if !ok {
			return fmt.Errorf("%q not found in artifact map", artifact)
		}

		err := os.Chmod(path, 0700)
		if err != nil {
			return fmt.Errorf("error setting execute permissions on artifact %s: %v", step.GetArtifactId(), err)
		}
	case step.GetLocalPath() != "":
		path = step.GetLocalPath()
	default:
		return fmt.Errorf("can't determine location type")

	}

	return executeCommand(path, step.Args, stepDir, runEnvs, []int32{0})
}

func runScript(step *agentendpointpb.SoftwareRecipe_Step_RunScript, artifacts map[string]string, runEnvs []string, stepDir string) error {
	var extension string
	if runtime.GOOS == "windows" {
		extension = extensionMap[step.Interpreter]
	}
	scriptPath := filepath.Join(stepDir, "recipe_script_source"+extension)
	if err := writeScript(scriptPath, step.Script); err != nil {
		return err
	}

	var cmd string
	var args []string
	switch step.Interpreter {
	case agentendpointpb.SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED:
		cmd = scriptPath
	case agentendpointpb.SoftwareRecipe_Step_RunScript_SHELL:
		if runtime.GOOS == "windows" {
			cmd = scriptPath
		} else {
			args = append([]string{scriptPath})
			cmd = "/bin/sh"
		}
	case agentendpointpb.SoftwareRecipe_Step_RunScript_POWERSHELL:
		if runtime.GOOS != "windows" {
			return fmt.Errorf("interpreter %q can only be used on Windows systems", step.Interpreter)
		}
		args = append([]string{"-File", scriptPath})
		cmd = "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\PowerShell.exe"
	default:
		return fmt.Errorf("unsupported interpreter %q", step.Interpreter)
	}
	return executeCommand(cmd, args, stepDir, runEnvs, step.AllowedExitCodes)
}

func writeScript(path, contents string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(contents); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(path, 0755); err != nil {
		return err
	}
	return nil
}

func executeCommand(cmd string, args []string, workDir string, runEnvs []string, allowedExitCodes []int32) error {
	cmdObj := exec.Command(cmd, args...)
	cmdObj.Dir = workDir
	defaultEnv, err := createDefaultEnvironment()
	if err != nil {
		return fmt.Errorf("error creating default environment: %v", err)
	}
	cmdObj.Env = append(cmdObj.Env, defaultEnv...)
	cmdObj.Env = append(cmdObj.Env, runEnvs...)

	o, err := cmdObj.CombinedOutput()
	logger.Infof("Combined output for %q command:\n%s", cmd, o)
	if err == nil {
		return nil
	}

	if v, ok := err.(*exec.ExitError); ok && len(allowedExitCodes) != 0 {
		result := int32(v.ExitCode())
		for _, code := range allowedExitCodes {
			if result == code {
				return nil
			}
		}
	}

	return err
}
