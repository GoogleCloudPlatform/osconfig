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
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

// StepFileCopy builds the command for a FileCopy step
func StepFileCopy(step *osconfigpb.SoftwareRecipe_Step_FileCopy, artifacts map[string]string) error {
	fmt.Println("StepFileCopy")
	return nil
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
func StepFileExec(step *osconfigpb.SoftwareRecipe_Step_FileExec, artifacts map[string]string, runEnvs []string, runDir string) error {
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

	return executeCommand(path, filepath.Join(runDir, "stepName"), runEnvs, step.FileExec.Args...)
}

// StepScriptRun builds the command for a ScriptRun step
func StepScriptRun(step *osconfigpb.SoftwareRecipe_Step_ScriptRun, artifacts map[string]string, runEnvs []string, runDir string) error {
	// TODO: should be putting this in stepN_type/ dir, but that needs me to know the dir way in advance..
	// actually this is an artifact. we should have made them referenced as artifacts.. we'll have to repeat file creation logic here.
	f, err := os.Create("/tmp/scriptrun")
	if err != nil {
		return err
	}
	defer f.Close()

	f.WriteString(step.ScriptRun.Script)
	f.Sync()
	if err := os.Chmod("/tmp/scriptrun", 0755); err != nil {
		return err
	}

	var qargs []string
	//if step.ScriptRun.Interpreter == osconfigpb.SoftwareRecipe_Step_RunScript_POWERSHELL {
	for _, arg := range step.ScriptRun.Args {
		qargs = append(qargs, fmt.Sprintf("%q", arg))
	}
	command := "/tmp/scriptrun" + " " + strings.Join(qargs, " ")
	args := []string{"-c", command}
	return executeCommand("/bin/sh", filepath.Join(runDir, "stepName"), runEnvs, args...)
}

func executeCommand(cmd string, workDir string, envs []string, args ...string) error {
	cmdObj := exec.Command(cmd, args...)

	cmdObj.Dir = workDir
	if err := os.MkdirAll(cmdObj.Dir, os.ModeDir|0755); err != nil {
		return fmt.Errorf("failed to create working dir %q: %s", workDir, err)
	}

	cmdObj.Env = append(cmdObj.Env, envs...)
	cmdObj.Env = append(cmdObj.Env, fmt.Sprintf("PWD=%s", cmdObj.Dir))

	// TODO: log output from command.
	_, err := cmdObj.Output()
	if err != nil {
		return err
	}
	return nil
}
