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
	"errors"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/config"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
)

var (
	winRoot = os.Getenv("SystemRoot")
	sh      = "/bin/sh"

	winPowershell string
	winCmd        string

	goos = runtime.GOOS

	errLinuxPowerShell = errors.New("interpreter POWERSHELL cannot be used on non-Windows system")
	errWinNoInt        = fmt.Errorf("interpreter must be specified for a Windows system")
)

type execTask struct {
	client *Client

	TaskID    string
	Task      *execStepTask
	StartedAt time.Time         `json:",omitempty"`
	LogLabels map[string]string `json:",omitempty"`
}

type execStepTask struct {
	*agentendpointpb.ExecStepTask
}

func (e *execTask) reportCompletedState(ctx context.Context, errMsg string, output *agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput) error {
	req := &agentendpointpb.ReportTaskCompleteRequest{
		TaskId:       e.TaskID,
		TaskType:     agentendpointpb.TaskType_EXEC_STEP_TASK,
		ErrorMessage: errMsg,
		Output:       output,
	}
	if err := e.client.ReportTaskComplete(ctx, req); err != nil {
		return fmt.Errorf("error reporting completed state: %v", err)
	}
	return nil
}

func (e *execTask) run(ctx context.Context) error {
	e.StartedAt = time.Now()
	req := &agentendpointpb.ReportTaskProgressRequest{
		TaskId:   e.TaskID,
		TaskType: agentendpointpb.TaskType_EXEC_STEP_TASK,
		Progress: &agentendpointpb.ReportTaskProgressRequest_ExecStepTaskProgress{
			ExecStepTaskProgress: &agentendpointpb.ExecStepTaskProgress{State: agentendpointpb.ExecStepTaskProgress_STARTED},
		},
	}
	res, err := e.client.ReportTaskProgress(ctx, req)
	if err != nil {
		return fmt.Errorf("error reporting state %s: %v", agentendpointpb.ExecStepTaskProgress_STARTED, err)
	}

	if res.GetTaskDirective() == agentendpointpb.TaskDirective_STOP {
		return e.reportCompletedState(ctx, errServerCancel.Error(), &agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput{
			// TODO: Maybe there should be a canceled state instead?
			ExecStepTaskOutput: &agentendpointpb.ExecStepTaskOutput{State: agentendpointpb.ExecStepTaskOutput_COMPLETED},
		})
	}

	return e.reportCompletedState(ctx, "", &agentendpointpb.ReportTaskCompleteRequest_ExecStepTaskOutput{
		ExecStepTaskOutput: &agentendpointpb.ExecStepTaskOutput{
			State: agentendpointpb.ExecStepTaskOutput_COMPLETED,
		},
	})
}

// RunExecStep runs an exec step task.
func (c *Client) RunExecStep(ctx context.Context, task *agentendpointpb.ExecStepTask) error {
	e := &execTask{
		client:    c,
		Task:      &execStepTask{task},
		LogLabels: map[string]string{"instance_name": config.Name(), "agent_version": config.Version()},
	}

	return e.run(ctx)
}
