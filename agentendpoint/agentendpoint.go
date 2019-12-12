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
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	agentendpoint "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/cloud.google.com/go/osconfig/agentendpoint/apiv1beta"
	"github.com/GoogleCloudPlatform/osconfig/config"
	"github.com/GoogleCloudPlatform/osconfig/inventory/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/tasker"
	"github.com/GoogleCloudPlatform/osconfig/util"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

const apiRetrySec = 600

var (
	errServerCancel = errors.New("task canceled by server")
	taskStateFile   = config.TaskStateFile()
)

// Client is a an agentendpoint client.
type Client struct {
	raw    *agentendpoint.Client
	cancel context.CancelFunc
	noti   chan struct{}
	closed bool
	mx     sync.Mutex
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

	return &Client{raw: c, noti: make(chan struct{}, 1)}, nil
}

// Close cancels WaitForTaskNotification and closes the underlying ClientConn.
func (c *Client) Close() error {
	// Lock so nothing can use the client while we are closing.
	c.mx.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.closed = true
	return c.raw.Close()
}

// Closed reports whether the Client has been closed.
func (c *Client) Closed() bool {
	return c.closed
}

func (c *Client) startNextTask(ctx context.Context) (res *agentendpointpb.StartNextTaskResponse, err error) {
	token, err := config.IDToken()
	if err != nil {
		return nil, err
	}

	req := &agentendpointpb.StartNextTaskRequest{}
	logger.Debugf("Calling StartNextTask with request:\n%s", util.PrettyFmt(req))
	req.InstanceIdToken = token

	if err := retryAPICall(apiRetrySec*time.Second, "StartNextTask", func() error {
		res, err = c.raw.StartNextTask(ctx, req)
		if err != nil {
			return err
		}
		logger.Debugf("StartNextTask response:\n%s", util.PrettyFmt(res))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("error calling StartNextTask: %v", err)
	}

	return res, nil
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

	return res, nil
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
func (c *Client) LookupEffectiveGuestPolicies(ctx context.Context) (res *agentendpointpb.EffectiveGuestPolicy, err error) {
	info, err := osinfo.Get()
	if err != nil {
		return nil, err
	}

	req := &agentendpointpb.LookupEffectiveGuestPolicyRequest{
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

	if err := retryAPICall(apiRetrySec*time.Second, "LookupEffectiveGuestPolicies", func() error {
		res, err = c.raw.LookupEffectiveGuestPolicy(ctx, req)
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
		res, err := c.startNextTask(ctx)
		if err != nil {
			logger.Errorf("Error running StartNextTask, cannot continue: %v", err)
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
			if err := c.RunApplyPatches(ctx, task); err != nil {
				logger.Errorf("Error running TaskType_APPLY_PATCHES: %v", err)
			}
		case agentendpointpb.TaskType_EXEC_STEP_TASK:
			if err := c.RunExecStep(ctx, task); err != nil {
				logger.Errorf("Error running TaskType_EXEC_STEP_TASK: %v", err)
			}
		default:
			logger.Errorf("Unknown task type: %v", task.GetTaskType())
		}
	}
}

func (c *Client) handleStream(ctx context.Context, stream agentendpointpb.AgentEndpointService_ReceiveTaskNotificationClient) error {
	for {
		logger.Debugf("Waiting on ReceiveTaskNotification stream Recv().")
		if _, err := stream.Recv(); err != nil {
			// Return on any stream error, even a close, the caller will simply
			// reconnect the stream as needed.
			return err
		}
		logger.Debugf("Received task notification.")

		// Only queue up one notifcation at a time. We should only ever
		// have one active task being worked on and one in the queue.
		select {
		case <-ctx.Done():
			// We have been canceled.
			return nil
		case c.noti <- struct{}{}:
			tasker.Enqueue("TaskNotification", func() {
				// We lock so that this task will complete before the client can get canceled.
				c.mx.Lock()
				defer c.mx.Unlock()
				select {
				case <-ctx.Done():
					// We have been canceled.
				default:
					// Take this task off the notification queue so another can be
					// queued up.
					<-c.noti
					c.runTask(ctx)
				}
			})
		default:
			// Ignore the notificaction as we already have one queued.
		}
	}
}

func (c *Client) receiveTaskNotification(ctx context.Context) (agentendpointpb.AgentEndpointService_ReceiveTaskNotificationClient, error) {
	req := &agentendpointpb.ReceiveTaskNotificationRequest{
		AgentVersion: config.Version(),
	}

	token, err := config.IDToken()
	if err != nil {
		return nil, fmt.Errorf("error fetching Instance IDToken: %v", err)
	}

	logger.Debugf("Calling ReceiveTaskNotification with request:\n%s", util.PrettyFmt(req))
	req.InstanceIdToken = token

	return c.raw.ReceiveTaskNotification(ctx, req)
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

var errServiceNotEnabled = errors.New("service is not enabled for this project")

func (c *Client) waitForTask(ctx context.Context) error {
	stream, err := c.receiveTaskNotification(ctx)
	if err != nil {
		return err
	}

	err = c.handleStream(ctx, stream)
	if err == io.EOF {
		// Server closed the stream indication we should reconnect.
		return nil
	}
	if s, ok := status.FromError(err); ok {
		switch s.Code() {
		case codes.Unavailable:
			// Something canceled the stream (could be deadline/timeout), we should reconnect.
			logger.Debugf("Stream canceled, will reconnect: %v", err)
			return nil
		case codes.PermissionDenied:
			// Service is not enabled for this project.
			return errServiceNotEnabled
		}
	}
	// TODO: Add more error checking (handle more API erros vs non API errors) and backoff where appropriate.
	return err
}

// WaitForTaskNotification waits for and acts on any task notification until the Client is closed.
// Multiple calls to WaitForTaskNotification will not create new watchers.
func (c *Client) WaitForTaskNotification(ctx context.Context) {
	c.mx.Lock()
	defer c.mx.Unlock()
	if c.cancel != nil {
		// WaitForTaskNotification is already running on this client.
		return
	}
	logger.Debugf("Running WaitForTaskNotification")
	ctx, c.cancel = context.WithCancel(ctx)

	logger.Debugf("Checking local state file for saved task.")
	if err := c.loadTaskFromState(ctx); err != nil {
		logger.Errorf(err.Error())
	}

	logger.Debugf("Setting up ReceiveTaskNotification stream watcher.")
	go func() {
		for {
			select {
			case <-ctx.Done():
				// We have been canceled.
				logger.Debugf("Disabling WaitForTaskNotification")
				return
			default:
			}

			if err := c.waitForTask(ctx); err != nil {
				if err == errServiceNotEnabled {
					time.Sleep(config.SvcPollInterval())
					continue
				}
				logger.Errorf("Error waiting for task: %v", err)
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

func mkLabels(task *agentendpointpb.Task) map[string]string {
	labels := task.GetServiceLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels["instance_name"] = config.Name()
	labels["agent_version"] = config.Version()
	return labels
}
