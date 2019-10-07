//  Copyright 2018 Google Inc. All Rights Reserved.
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

package ospatchog

import (
	"context"
	"fmt"
	"os"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

func (r *patchRun) execPreStep() error {
	logger := &util.Logger{Debugf: r.debugf, Infof: r.infof, Warningf: r.warningf, Errorf: r.errorf, Fatalf: nil}
	return execStep(r.ctx, logger, r.Job.GetPatchConfig().GetPreStep().GetWindowsExecStepConfig())
}

func (r *patchRun) execPostStep() error {
	logger := &util.Logger{Debugf: r.debugf, Infof: r.infof, Warningf: r.warningf, Errorf: r.errorf, Fatalf: nil}
	return execStep(r.ctx, logger, r.Job.GetPatchConfig().GetPostStep().GetWindowsExecStepConfig())
}

func execStep(ctx context.Context, logger *util.Logger, stepConfig *osconfigpb.ExecStepConfig) error {
	if stepConfig != nil {
		localPath, err := getExecutablePath(ctx, logger, stepConfig)
		if err != nil {
			return fmt.Errorf("error getting executable path: %v", err)
		}

		codes := stepConfig.GetAllowedSuccessCodes()

		switch stepConfig.GetInterpreter() {
		case osconfigpb.ExecStepConfig_INTERPRETER_UNSPECIFIED:
			err = fmt.Errorf("interpreter must be specified for a Windows system")
		case osconfigpb.ExecStepConfig_SHELL:
			err = executeCommand(logger, "C:\\Windows\\System32\\cmd.exe", codes, "/c", localPath)
		case osconfigpb.ExecStepConfig_POWERSHELL:
			err = executeCommand(logger, "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\PowerShell.exe", codes, "-File", localPath)
		default:
			err = fmt.Errorf("invalid interpreter %q", stepConfig.GetInterpreter())
		}

		if gcsObject := stepConfig.GetGcsObject(); gcsObject != nil {
			if err := os.Remove(localPath); err != nil {
				logger.Errorf("error removing downloaded file %s", err)
			}
		}

		return err
	}

	logger.Debugf("No ExecStepConfig for Windows")
	return nil
}
