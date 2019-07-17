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
	"strconv"
	"strings"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

// BuildCommand builds a command []string based on the Step type and parameters.
func BuildCommand(step *osconfigpb.SoftwareRecipe_Step, artifacts map[string]string) ([]string, error) {
	switch v := step.Step.(type) {
	case *osconfigpb.SoftwareRecipe_Step_FileCopy:
		return StepFileCopy(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_ArchiveExtraction:
		return StepArchiveExtraction(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_MsiInstallation:
		return StepMsiInstallation(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_DpkgInstallation:
		return StepDpkgInstallation(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_RpmInstallation:
		return StepRpmInstallation(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_FileExec:
		return StepFileExec(v, artifacts)
	case *osconfigpb.SoftwareRecipe_Step_ScriptRun:
		return StepScriptRun(v, artifacts)
	default:
		return nil, fmt.Errorf("unknown step type %T", v)
	}
}

// StepFileCopy builds the command for a FileCopy step
func StepFileCopy(step *osconfigpb.SoftwareRecipe_Step_FileCopy, artifacts map[string]string) ([]string, error) {
	dest, err := normPath(step.FileCopy.Destination)
	if err != nil {
		return nil, err
	}

	permissions, err := parsePermissions(step.FileCopy.Permissions)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(dest); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
	} else {
		// file exists
		if !step.FileCopy.Overwrite {
			logger.Infof("skipping FileCopy step as file at %s already exists", dest)
			return nil, nil
		}
		os.Chmod(dest, permissions)
	}

	src, ok := artifacts[step.FileCopy.ArtifactId]
	if !ok {
		return nil, fmt.Errorf("Could not find location for artifact %q in FileCopy step", step.FileCopy.ArtifactId)
	}

	reader, err := os.Open(src)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	writer, err := os.OpenFile(dest, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, permissions)
	if err != nil {
		return nil, err
	}
	defer writer.Close()

	_, err = io.Copy(writer, reader)
	if err != nil {
		return nil, err
	}

	return nil, nil
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
func StepArchiveExtraction(step *osconfigpb.SoftwareRecipe_Step_ArchiveExtraction, artifacts map[string]string) ([]string, error) {
	fmt.Println("StepArchiveExtraction")
	return nil, nil
}

// StepMsiInstallation builds the command for a MsiInstallation step
func StepMsiInstallation(step *osconfigpb.SoftwareRecipe_Step_MsiInstallation, artifacts map[string]string) ([]string, error) {
	fmt.Println("StepMsiInstallation")
	return nil, nil
}

// StepDpkgInstallation builds the command for a DpkgInstallation step
func StepDpkgInstallation(step *osconfigpb.SoftwareRecipe_Step_DpkgInstallation, artifacts map[string]string) ([]string, error) {
	fmt.Println("StepDpkgInstallation")
	return nil, nil
}

// StepRpmInstallation builds the command for a FileCopy step
func StepRpmInstallation(step *osconfigpb.SoftwareRecipe_Step_RpmInstallation, artifacts map[string]string) ([]string, error) {
	fmt.Println("StepRpmInstallation")
	return nil, nil
}

// StepFileExec builds the command for a FileExec step
func StepFileExec(step *osconfigpb.SoftwareRecipe_Step_FileExec, artifacts map[string]string) ([]string, error) {
	var path string
	switch v := step.FileExec.LocationType.(type) {
	case *osconfigpb.SoftwareRecipe_Step_ExecFile_LocalPath:
		path = v.LocalPath
	case *osconfigpb.SoftwareRecipe_Step_ExecFile_ArtifactId:
		var ok bool
		path, ok = artifacts[v.ArtifactId]
		if !ok {
			return nil, fmt.Errorf("%q not found in artifact map", v.ArtifactId)
		}
	default:
		return nil, fmt.Errorf("can't determine location type")
	}

	res := []string{path}
	res = append(res, step.FileExec.Args...)
	return res, nil
}

// StepScriptRun builds the command for a ScriptRun step
func StepScriptRun(step *osconfigpb.SoftwareRecipe_Step_ScriptRun, artifacts map[string]string) ([]string, error) {
	// TODO: should be putting this in stepN_type/ dir, but that needs me to know the dir way in advance..
	// actually this is an artifact. we should have made them referenced as artifacts.. we'll have to repeat file creation logic here.
	f, err := os.Create("/tmp/scriptrun")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	f.WriteString(step.ScriptRun.Script)
	f.Sync()
	if err := os.Chmod("/tmp/scriptrun", 0755); err != nil {
		return nil, err
	}

	res := []string{"/bin/sh", "-c"}
	//if step.ScriptRun.Interpreter == osconfigpb.SoftwareRecipe_Step_RunScript_POWERSHELL {
	var qargs []string
	for _, arg := range step.ScriptRun.Args {
		qargs = append(qargs, fmt.Sprintf("%q", arg))
	}
	res = append(res, "/tmp/scriptrun"+" "+strings.Join(qargs, " "))
	return res, nil
}
