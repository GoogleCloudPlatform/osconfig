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

package agentendpoint

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

type execTaskBeta struct {
	client *BetaClient

	TaskID    string
	Task      *execStepTaskBeta
	StartedAt time.Time         `json:",omitempty"`
	LogLabels map[string]string `json:",omitempty"`
}

type execStepTaskBeta struct {
	*agentendpointpb.ExecStepTask
}

func (e *execTaskBeta) reportCompletedState(ctx context.Context, errMsg string, output *agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput) error {
	req := &agentendpointpb.ReportTaskCompleteRequest{
		TaskId:       e.TaskID,
		TaskType:     agentendpointpb.TaskType_EXEC_STEP_TASK,
		ErrorMessage: errMsg,
		Output:       output,
	}
	if err := e.client.reportTaskComplete(ctx, req); err != nil {
		return fmt.Errorf("error reporting completed state: %v", err)
	}
	return nil
}

func (e *execTaskBeta) run(ctx context.Context) error {
	e.StartedAt = time.Now()
	req := &agentendpointpb.ReportTaskProgressRequest{
		TaskId:   e.TaskID,
		TaskType: agentendpointpb.TaskType_EXEC_STEP_TASK,
		Progress: &agentendpointpb.ReportTaskProgressRequest_ExecStepTaskProgress{
			ExecStepTaskProgress: &agentendpointpb.ExecStepTaskProgress{State: agentendpointpb.ExecStepTaskProgress_STARTED},
		},
	}
	res, err := e.client.reportTaskProgress(ctx, req)
	if err != nil {
		return fmt.Errorf("error reporting state %s: %v", agentendpointpb.ExecStepTaskProgress_STARTED, err)
	}

	if res.GetTaskDirective() == agentendpointpb.TaskDirective_STOP {
		return e.reportCompletedState(ctx, errServerCancel.Error(), &agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput{
			ExecStepTaskOutput: &agentendpointpb.ExecStepTaskOutput{State: agentendpointpb.ExecStepTaskOutput_CANCELLED},
		})
	}

	stepConfig := e.Task.GetExecStep().GetLinuxExecStepConfig()
	if goos == "windows" {
		stepConfig = e.Task.GetExecStep().GetWindowsExecStepConfig()
	}

	localPath := stepConfig.GetLocalPath()
	if gcsObject := stepConfig.GetGcsObject(); gcsObject != nil {
		var err error
		localPath, err = getGCSObject(ctx, gcsObject.GetBucket(), gcsObject.GetObject(), gcsObject.GetGenerationNumber(), e.LogLabels)
		if err != nil {
			return fmt.Errorf("error getting executable path: %v", err)
		}
		defer func() {
			if err := os.Remove(localPath); err != nil {
				logger.Errorf("error removing downloaded file %s", err)
			}
		}()
	}

	exitCode := int32(-1)
	switch stepConfig.GetInterpreter() {
	case agentendpointpb.ExecStepConfig_INTERPRETER_UNSPECIFIED:
		if goos == "windows" {
			err = errWinNoInt
		} else {
			exitCode, err = executeCommand(localPath, nil, e.LogLabels)
		}
	case agentendpointpb.ExecStepConfig_SHELL:
		if goos == "windows" {
			exitCode, err = executeCommand(winCmd, []string{"/c", localPath}, e.LogLabels)
		} else {
			exitCode, err = executeCommand(sh, []string{localPath}, e.LogLabels)
		}
	case agentendpointpb.ExecStepConfig_POWERSHELL:
		if goos == "windows" {
			exitCode, err = executeCommand(winPowershell, append(winPowershellArgs, "-File", localPath), e.LogLabels)
		} else {
			err = errLinuxPowerShell
		}
	default:
		err = fmt.Errorf("invalid interpreter %q", stepConfig.GetInterpreter())
	}
	if err != nil {
		return e.reportCompletedState(ctx, err.Error(), &agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput{
			ExecStepTaskOutput: &agentendpointpb.ExecStepTaskOutput{
				State:    agentendpointpb.ExecStepTaskOutput_COMPLETED,
				ExitCode: exitCode,
			},
		})
	}

	return e.reportCompletedState(ctx, "", &agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput{
		ExecStepTaskOutput: &agentendpointpb.ExecStepTaskOutput{
			State:    agentendpointpb.ExecStepTaskOutput_COMPLETED,
			ExitCode: exitCode,
		},
	})
}

// RunExecStep runs an exec step task.
func (c *BetaClient) RunExecStep(ctx context.Context, task *agentendpointpb.Task) error {
	e := &execTaskBeta{
		TaskID:    task.GetTaskId(),
		client:    c,
		Task:      &execStepTaskBeta{task.GetExecStepTask()},
		LogLabels: mkLabels(task.GetServiceLabels()),
	}

	return e.run(ctx)
}
