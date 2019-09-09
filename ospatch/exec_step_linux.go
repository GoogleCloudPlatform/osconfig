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

package ospatch

import (
	"context"
	"fmt"
	"os"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
	"github.com/GoogleCloudPlatform/osconfig/util"
)

func (r *patchRun) execPreStep() error {
	logger := &util.Logger{Debugf: r.debugf, Infof: r.infof, Warningf: r.warningf, Errorf: r.errorf, Fatalf: nil}
	return execStep(r.ctx, logger, r.Job.GetPatchConfig().GetPreStep().GetLinuxExecStepConfig())
}

func (r *patchRun) execPostStep() error {
	logger := &util.Logger{Debugf: r.debugf, Infof: r.infof, Warningf: r.warningf, Errorf: r.errorf, Fatalf: nil}
	return execStep(r.ctx, logger, r.Job.GetPatchConfig().GetPostStep().GetLinuxExecStepConfig())
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
			err = executeCommand(logger, localPath, codes)
		case osconfigpb.ExecStepConfig_SHELL:
			err = executeCommand(logger, "/bin/sh", codes, localPath)
		case osconfigpb.ExecStepConfig_POWERSHELL:
			err = fmt.Errorf("interpreter POWERSHELL cannot be used on non-Windows system")
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
	return nil
}
