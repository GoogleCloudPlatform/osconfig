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
	"time"

	"github.com/GoogleCloudPlatform/osconfig/clog"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/internal/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
)

type configTask struct {
	client *Client

	TaskID    string
	Task      *applyConfigTask
	StartedAt time.Time `json:",omitempty"`

	// ApplyConfigTaskOutput result
	result *agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults_
}

type applyConfigTask struct {
	*agentendpointpb.ApplyConfigTask
}

func (c *configTask) reportCompletedState(ctx context.Context, errMsg string, output *agentendpointpb.ApplyConfigTaskOutput) error {
	req := &agentendpointpb.ReportTaskCompleteRequest{
		TaskId:       c.TaskID,
		TaskType:     agentendpointpb.TaskType_APPLY_CONFIG_TASK,
		ErrorMessage: errMsg,
		Output: &agentendpointpb.ReportTaskCompleteRequest_ApplyConfigTaskOutput{
			ApplyConfigTaskOutput: output,
		},
	}
	if err := c.client.reportTaskComplete(ctx, req); err != nil {
		return fmt.Errorf("error reporting completed state: %v", err)
	}
	return nil
}

func (c *configTask) reportContinuingState(ctx context.Context, configState agentendpointpb.ApplyConfigTaskProgress_State) error {
	req := &agentendpointpb.ReportTaskProgressRequest{
		TaskId:   c.TaskID,
		TaskType: agentendpointpb.TaskType_APPLY_PATCHES,
		Progress: &agentendpointpb.ReportTaskProgressRequest_ApplyConfigTaskProgress{
			ApplyConfigTaskProgress: &agentendpointpb.ApplyConfigTaskProgress{State: configState},
		},
	}
	res, err := c.client.reportTaskProgress(ctx, req)
	if err != nil {
		return fmt.Errorf("error reporting state %s: %v", configState, err)
	}
	if res.GetTaskDirective() == agentendpointpb.TaskDirective_STOP {
		return errServerCancel
	}
	return nil
}

func (c *configTask) reportFailed(ctx context.Context, msg string, failureResult *agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults_) error {
	clog.Errorf(ctx, msg)
	return c.reportCompletedState(ctx, msg, &agentendpointpb.ApplyConfigTaskOutput{Result: failureResult})
}

func (c *configTask) run(ctx context.Context) error {
	c.StartedAt = time.Now()
	if err := c.reportContinuingState(ctx, agentendpointpb.ApplyConfigTaskProgress_STARTED); err != nil {
		c.reportFailed(ctx, "", &agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults_{})
	}

	return c.reportCompletedState(ctx, "", &agentendpointpb.ApplyConfigTaskOutput{
		Result: &agentendpointpb.ApplyConfigTaskOutput_ConfigAssignmentResults_{},
	})
}

// RunApplyConfig runs an apply config task.
func (c *Client) RunApplyConfig(ctx context.Context, task *agentendpointpb.Task) error {
	ctx = clog.WithLabels(ctx, task.GetServiceLabels())
	e := &configTask{
		TaskID: task.GetTaskId(),
		client: c,
		Task:   &applyConfigTask{task.GetApplyConfigTask()},
	}

	return e.run(ctx)
}
