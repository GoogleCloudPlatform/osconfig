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
	"errors"
	"fmt"
	"runtime"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

func stepInstallMsi(step *agentendpointpb.SoftwareRecipe_Step_InstallMsi, artifacts map[string]string, runEnvs []string, stepDir string) error {
	if runtime.GOOS != "windows" {
		return errors.New("SoftwareRecipe_Step_InstallMsi only applicable on Windows")
	}
	artifact := step.GetArtifactId()
	path, ok := artifacts[artifact]
	if !ok {
		return fmt.Errorf("%q not found in artifact map", artifact)
	}
	args := step.Flags
	if len(args) == 0 {
		args = []string{"/i", "/qn", "/norestart"}
	}
	args = append(args, path)

	exitCodes := step.AllowedExitCodes
	if len(exitCodes) == 0 {
		exitCodes = []int32{0, 1641, 3010}
	}
	return executeCommand("C:\\Windows\\System32\\msiexec.exe", args, stepDir, runEnvs, exitCodes)
}
