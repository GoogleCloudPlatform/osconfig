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

package recipes

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/GoogleCloudPlatform/osconfig/common"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

// StepFileCopy builds the command for a FileCopy step
func StepFileCopy(step *osconfigpb.SoftwareRecipe_Step_FileCopy, artifacts map[string]string) error {
	dest, err := common.NormPath(step.FileCopy.Destination)
	if err != nil {
		return err
	}

	permissions, err := parsePermissions(step.FileCopy.Permissions)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dest); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		// file exists
		if !step.FileCopy.Overwrite {
			return fmt.Errorf("file already exists at path %q and Overwrite = false", step.FileCopy.Destination)
		}
		os.Chmod(dest, permissions)
	}

	src, ok := artifacts[step.FileCopy.ArtifactId]
	if !ok {
		return fmt.Errorf("could not find location for artifact %q", step.FileCopy.ArtifactId)
	}

	reader, err := os.Open(src)
	if err != nil {
		return err
	}
	defer reader.Close()
	writer, err := os.OpenFile(dest, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, permissions)
	if err != nil {
		return err
	}
	defer writer.Close()

	_, err = io.Copy(writer, reader)
	if err != nil {
		return err
	}

	return nil
}

func parsePermissions(s string) (os.FileMode, error) {
	if s == "" {
		return 755, nil
	}

	i, err := strconv.ParseUint(s, 8, 32)
	if err != nil {
		return 0, err
	}
	return os.FileMode(i), nil
}

// StepArchiveExtraction builds the command for a ArchiveExtraction step
func StepArchiveExtraction(step *osconfigpb.SoftwareRecipe_Step_ArchiveExtraction, artifacts map[string]string) error {
	fmt.Println("StepArchiveExtraction")
	return nil
}

// StepMsiInstallation builds the command for a MsiInstallation step
func StepMsiInstallation(step *osconfigpb.SoftwareRecipe_Step_MsiInstallation, artifacts map[string]string) error {
	fmt.Println("StepMsiInstallation")
	return nil
}

// StepDpkgInstallation builds the command for a DpkgInstallation step
func StepDpkgInstallation(step *osconfigpb.SoftwareRecipe_Step_DpkgInstallation, artifacts map[string]string) error {
	fmt.Println("StepDpkgInstallation")
	return nil
}

// StepRpmInstallation builds the command for a FileCopy step
func StepRpmInstallation(step *osconfigpb.SoftwareRecipe_Step_RpmInstallation, artifacts map[string]string) error {
	fmt.Println("StepRpmInstallation")
	return nil
}

// StepFileExec builds the command for a FileExec step
func StepFileExec(step *osconfigpb.SoftwareRecipe_Step_FileExec, artifacts map[string]string, runEnvs []string, stepDir string) error {
	var path string
	switch v := step.FileExec.LocationType.(type) {
	case *osconfigpb.SoftwareRecipe_Step_ExecFile_LocalPath:
		path = v.LocalPath
	case *osconfigpb.SoftwareRecipe_Step_ExecFile_ArtifactId:
		var ok bool
		path, ok = artifacts[v.ArtifactId]
		if !ok {
			return fmt.Errorf("%q not found in artifact map", v.ArtifactId)
		}
	default:
		return fmt.Errorf("can't determine location type")
	}

	return executeCommand(path, step.FileExec.Args, stepDir, runEnvs, []int32{0})
}

// StepScriptRun runs a ScriptRun step.
func StepScriptRun(step *osconfigpb.SoftwareRecipe_Step_ScriptRun, artifacts map[string]string, runEnvs []string, stepDir string) error {
	cmd := filepath.Join(stepDir, "recipe_script_source")
	if err := writeScript(cmd, step.ScriptRun.Script); err != nil {
		return err
	}

	var args []string
	switch step.ScriptRun.Interpreter {
	case osconfigpb.SoftwareRecipe_Step_RunScript_INTERPRETER_UNSPECIFIED:
		if runtime.GOOS == "windows" {
			args = []string{"/c", cmd}
			cmd = "C:\\Windows\\System32\\cmd.exe"
		}
	case osconfigpb.SoftwareRecipe_Step_RunScript_SHELL:
		if runtime.GOOS == "windows" {
			args = []string{"/c", cmd}
			cmd = "C:\\Windows\\System32\\cmd.exe"
		}
		args = []string{"-c", cmd}
		cmd = "/bin/sh"
	case osconfigpb.SoftwareRecipe_Step_RunScript_POWERSHELL:
		if runtime.GOOS != "windows" {
			return fmt.Errorf("interpreter %q can only used on Windows systems", step.ScriptRun.Interpreter)
		}
		args = []string{"-File", cmd}
		cmd = "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\PowerShell.exe"
	default:
		return fmt.Errorf("unsupported interpreter %q", step.ScriptRun.Interpreter)
	}
	return executeCommand(cmd, args, stepDir, runEnvs, step.ScriptRun.AllowedExitCodes)
}

func writeScript(path, contents string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	f.WriteString(contents)
	err = f.Close()
	if err != nil {
		return err
	}
	err = os.Chmod(path, 0755)
	if err != nil {
		return err
	}
	return nil
}

func executeCommand(cmd string, args []string, workDir string, runEnvs []string, allowedExitCodes []int32) error {
	cmdObj := exec.Command(cmd, args...)
	cmdObj.Dir = workDir
	cmdObj.Env = append(cmdObj.Env, runEnvs...)

	// TODO: log output from command.
	_, err := cmdObj.Output()
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
