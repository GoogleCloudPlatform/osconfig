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

// Package agentendpoint connects to the osconfig agentendpoint api.
package agentendpoint

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	agentendpoint "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/cloud.google.com/go/osconfig/agentendpoint/apiv1alpha1"
	"github.com/GoogleCloudPlatform/osconfig/config"
	"github.com/GoogleCloudPlatform/osconfig/tasker"
	"github.com/GoogleCloudPlatform/osconfig/util"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
)

var (
	errServerCancel = errors.New("task canceled by server")
	taskStateFile   = config.TaskStateFile()
	patchEnabled    = config.OSPatchEnabled()
)

// Client is a an agentendpoint client.
type Client struct {
	raw *agentendpoint.Client
}

var client *Client

// NewClient a new agentendpoint Client.
func NewClient(ctx context.Context) (*Client, error) {
	c, err := agentendpoint.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	client = &Client{raw: c}
	return client, nil
}

// ReportTaskProgress calls the agentendpoint service ReportTaskProgress call injecting the InstanceIdToken.
func (c *Client) ReportTaskProgress(ctx context.Context, req *agentendpointpb.ReportTaskProgressRequest) (res *agentendpointpb.ReportTaskProgressResponse, err error) {
	logger.Debugf("ReportTaskProgress request:\n%s", util.PrettyFmt(req))
	token, err := config.IDToken()
	if err != nil {
		return nil, err
	}
	req.InstanceIdToken = token
	if err := retryAPICall(2100*time.Second, "ReportTaskProgress", func() error {
		res, err = c.raw.ReportTaskProgress(ctx, req)
		if err != nil {
			return err
		}
		logger.Debugf("ReportTaskProgress response:\n%s", util.PrettyFmt(res))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("error calling ReportTaskProgress: %v", err)
	}

	return
}

// ReportTaskComplete calls the agentendpoint service ReportTaskComplete call injecting the InstanceIdToken.
func (c *Client) ReportTaskComplete(ctx context.Context, req *agentendpointpb.ReportTaskCompleteRequest) error {
	logger.Debugf("ReportTaskComplete request:\n%s", util.PrettyFmt(req))
	token, err := config.IDToken()
	if err != nil {
		return err
	}
	req.InstanceIdToken = token
	if err := retryAPICall(2100*time.Second, "ReportTaskComplete", func() error {
		res, err := c.raw.ReportTaskComplete(ctx, req)
		if err != nil {
			return err
		}
		logger.Debugf("ReportTaskComplete response:\n%s", util.PrettyFmt(res))
		return nil
	}); err != nil {
		return fmt.Errorf("error calling ReportTaskComplete: %v", err)
	}

	return nil
}

func (c *Client) runTask(ctx context.Context) {
	for {
		token, err := config.IDToken()
		if err != nil {
			logger.Errorf(err.Error())
			return
		}

		start := &agentendpointpb.ReportTaskStartRequest{
			InstanceIdToken: token,
		}
		startRes, err := c.raw.ReportTaskStart(ctx, start)
		if err != nil {
			logger.Errorf(err.Error())
			return
		}

		task := startRes.GetTask()
		if task == nil {
			return
		}

		switch task.GetTaskType() {
		case agentendpointpb.TaskType_APPLY_PATCHES:
			if !patchEnabled {
				logger.Infof("Recieved TaskType_APPLY_PATCHES but OSPatch is not enabled")
				continue
			}

			if err := c.RunApplyPatches(ctx, task.GetApplyPatchesTask()); err != nil {
				logger.Errorf(err.Error())
			}
		case agentendpointpb.TaskType_EXEC_STEP_TASK:
			if err := c.RunExecStep(ctx, task.GetExecStepTask()); err != nil {
				logger.Errorf(err.Error())
			}
		default:
			logger.Errorf("Unknown task type: %v", task.GetTaskType())
		}
	}
}

func (c *Client) handleStream(ctx context.Context, stream agentendpointpb.AgentEndpointService_ReceiveTaskNotificationClient, noti chan struct{}) error {
	for {
		if _, err := stream.Recv(); err != nil {
			// Return on any stream error, even a close, the caller will simply
			// reconnect the stream as needed.
			return err
		}

		// Only queue up one notifcation at a time. We should only ever
		// have one active task being worked on and one in the queue.
		select {
		case noti <- struct{}{}:
			tasker.Enqueue("TaskNotification", func() {
				// Take this task off the notification queue so another can be
				// queued up.
				<-noti
				c.runTask(ctx)
			})
		default:
			// Ignore the notificaction as we already have one queued.
		}
	}
}

func (c *Client) watchStream(ctx context.Context, noti chan struct{}, token string) error {
	req := &agentendpointpb.ReceiveTaskNotificationRequest{
		AgentVersion:    config.Version(),
		InstanceIdToken: token,
	}

	stream, err := c.raw.ReceiveTaskNotification(ctx, req)
	if err != nil {
		return err
	}

	if err := c.handleStream(ctx, stream, noti); err != nil {
		if err == io.EOF {
			// Server closed the stream indication we should reconnect.
			return nil
		}
		logger.Errorf("error recieving on stream: %v", err)
	}

	return nil
}

func (c *Client) loadTaskFromState(ctx context.Context) error {
	st, err := loadState(taskStateFile)
	if err != nil {
		return fmt.Errorf("loadState error: %v", err)
	}
	if st != nil && st.PatchTask != nil {
		st.PatchTask.client = c
		tasker.Enqueue("PatchRun", func() {
			st.PatchTask.run(ctx)
		})
	}

	return nil
}

// WaitForTaskNotification waits for and acts on any task notification indefinitely.
func (c *Client) WaitForTaskNotification(ctx context.Context) error {
	if err := c.loadTaskFromState(ctx); err != nil {
		logger.Errorf(err.Error())
	}

	noti := make(chan struct{}, 1)
	for {
		token, err := config.IDToken()
		if err != nil {
			return err
		}

		if err := c.watchStream(ctx, noti, token); err != nil {
			return err
		}
	}
}
