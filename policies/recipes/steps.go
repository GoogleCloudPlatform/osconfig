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
	"strconv"
	"strings"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

// StepFileCopy builds the command for a FileCopy step
func StepFileCopy(step *osconfigpb.SoftwareRecipe_Step_FileCopy, artifacts map[string]string) error {
	dest, err := normPath(step.FileCopy.Destination)
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

	return executeCommand(path, stepDir, runEnvs, step.FileExec.Args...)
}

// StepScriptRun builds the command for a ScriptRun step
func StepScriptRun(step *osconfigpb.SoftwareRecipe_Step_ScriptRun, artifacts map[string]string, runEnvs []string, stepDir string) error {
	scriptPath := filepath.Join(stepDir, "script")

	f, err := os.Create(scriptPath)
	if err != nil {
		return err
	}

	f.WriteString(step.ScriptRun.Script)
	f.Close()
	if err := os.Chmod("/tmp/scriptrun", 0755); err != nil {
		return err
	}

	var qargs []string
	//if step.ScriptRun.Interpreter == osconfigpb.SoftwareRecipe_Step_RunScript_POWERSHELL {
	for _, arg := range step.ScriptRun.Args {
		qargs = append(qargs, fmt.Sprintf("%q", arg))
	}
	command := scriptPath + " " + strings.Join(qargs, " ")
	args := []string{"-c", command}
	return executeCommand("/bin/sh", stepDir, runEnvs, args...)
}

func executeCommand(cmd string, workDir string, runEnvs []string, args ...string) error {
	cmdObj := exec.Command(cmd, args...)

	cmdObj.Dir = workDir
	cmdObj.Env = append(cmdObj.Env, runEnvs...)

	// TODO: log output from command.
	_, err := cmdObj.Output()
	if err != nil {
		return err
	}
	return nil
}
