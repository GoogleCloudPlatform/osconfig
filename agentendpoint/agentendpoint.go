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
	"github.com/GoogleCloudPlatform/osconfig/inventory/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/tasker"
	"github.com/GoogleCloudPlatform/osconfig/util"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
)

const apiRetrySec = 600

var (
	errServerCancel = errors.New("task canceled by server")
	taskStateFile   = config.TaskStateFile()
)

// Client is a an agentendpoint client.
type Client struct {
	raw *agentendpoint.Client
}

// NewClient a new agentendpoint Client.
func NewClient(ctx context.Context) (*Client, error) {
	opts := []option.ClientOption{
		option.WithoutAuthentication(), // Do not use oauth.
		option.WithGRPCDialOption(grpc.WithTransportCredentials(credentials.NewTLS(nil))), // Because we disabled Auth we need to specifically enable TLS.
		option.WithEndpoint(config.SvcEndpoint()),
	}
	logger.Debugf("Creating new agentendpoint client.")
	c, err := agentendpoint.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{raw: c}, nil
}

// Close closes the underlying ClientConn.
func (c *Client) Close() error {
	return c.raw.Close()
}

func (c *Client) reportTaskStart(ctx context.Context) (res *agentendpointpb.ReportTaskStartResponse, err error) {
	token, err := config.IDToken()
	if err != nil {
		return nil, err
	}

	req := &agentendpointpb.ReportTaskStartRequest{}
	logger.Debugf("Calling ReportTaskStart with request:\n%s", util.PrettyFmt(req))
	req.InstanceIdToken = token

	if err := retryAPICall(apiRetrySec*time.Second, "ReportTaskStart", func() error {
		res, err = c.raw.ReportTaskStart(ctx, req)
		if err != nil {
			return err
		}
		logger.Debugf("ReportTaskStart response:\n%s", util.PrettyFmt(res))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("error calling ReportTaskStart: %v", err)
	}

	return
}

func (c *Client) reportTaskProgress(ctx context.Context, req *agentendpointpb.ReportTaskProgressRequest) (res *agentendpointpb.ReportTaskProgressResponse, err error) {
	token, err := config.IDToken()
	if err != nil {
		return nil, err
	}

	logger.Debugf("Calling ReportTaskProgress with request:\n%s", util.PrettyFmt(req))
	req.InstanceIdToken = token

	if err := retryAPICall(apiRetrySec*time.Second, "ReportTaskProgress", func() error {
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

func (c *Client) reportTaskComplete(ctx context.Context, req *agentendpointpb.ReportTaskCompleteRequest) error {
	token, err := config.IDToken()
	if err != nil {
		return err
	}

	logger.Debugf("Calling ReportTaskComplete with request:\n%s", util.PrettyFmt(req))
	req.InstanceIdToken = token

	if err := retryAPICall(apiRetrySec*time.Second, "ReportTaskComplete", func() error {
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

// LookupEffectiveGuestPolicies calls the agentendpoint service LookupEffectiveGuestPolicies.
func (c *Client) LookupEffectiveGuestPolicies(ctx context.Context) (*agentendpointpb.LookupEffectiveGuestPoliciesResponse, error) {
	info, err := osinfo.Get()
	if err != nil {
		return nil, err
	}

	req := &agentendpointpb.LookupEffectiveGuestPoliciesRequest{
		OsShortName:    info.ShortName,
		OsVersion:      info.Version,
		OsArchitecture: info.Architecture,
	}

	token, err := config.IDToken()
	if err != nil {
		return nil, err
	}

	logger.Debugf("Calling LookupEffectiveGuestPolicies with request:\n%s", util.PrettyFmt(req))
	req.InstanceIdToken = token

	var res *agentendpointpb.LookupEffectiveGuestPoliciesResponse
	if err := retryAPICall(apiRetrySec*time.Second, "LookupEffectiveGuestPolicies", func() error {
		res, err := c.raw.LookupEffectiveGuestPolicies(ctx, req)
		if err != nil {
			return err
		}
		logger.Debugf("LookupEffectiveGuestPolicies response:\n%s", util.PrettyFmt(res))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("error calling LookupEffectiveGuestPolicies: %v", err)
	}
	return res, nil
}

func (c *Client) runTask(ctx context.Context) {
	logger.Debugf("Beginning run task loop.")
	for {
		res, err := c.reportTaskStart(ctx)
		if err != nil {
			logger.Errorf("Error running ReportTaskStart, cannot continue: %v", err)
			return
		}

		task := res.GetTask()
		if task == nil {
			logger.Debugf("No task to run, ending run task loop.")
			return
		}

		logger.Debugf("Received task: %s.", task.GetTaskType())
		switch task.GetTaskType() {
		case agentendpointpb.TaskType_APPLY_PATCHES:
			if err := c.RunApplyPatches(ctx, task.GetTaskId(), task.GetApplyPatchesTask()); err != nil {
				logger.Errorf("Error running TaskType_APPLY_PATCHES: %v", err)
			}
		case agentendpointpb.TaskType_EXEC_STEP_TASK:
			if err := c.RunExecStep(ctx, task.GetTaskId(), task.GetExecStepTask()); err != nil {
				logger.Errorf("Error running TaskType_EXEC_STEP_TASK: %v", err)
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

	err = c.handleStream(ctx, stream, noti)
	if err != nil {
		if err == io.EOF {
			return nil
		}
		if s, ok := status.FromError(err); ok && s.Code() == codes.Unavailable {
			// Server closed the stream indication we should reconnect.
			return nil
		}
	}

	return err
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
	logger.Debugf("Checking local state file for saved task.")
	if err := c.loadTaskFromState(ctx); err != nil {
		logger.Errorf(err.Error())
	}

	logger.Debugf("Setting up ReceiveTaskNotification stream watcher.")
	noti := make(chan struct{}, 1)
	for {
		token, err := config.IDToken()
		if err != nil {
			return fmt.Errorf("error fetching Instance IDToken: %v", err)
		}

		if err := c.watchStream(ctx, noti, token); err != nil {
			if s, ok := status.FromError(err); ok && s.Code() == codes.PermissionDenied {
				// Service is not enabled for this project.
				time.Sleep(config.SvcPollInterval())
				continue
			}
			logger.Errorf("Error watching stream: %v", err)
			time.Sleep(5 * time.Second)
		}
	}
}
