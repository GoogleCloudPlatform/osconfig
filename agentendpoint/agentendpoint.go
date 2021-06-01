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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"cloud.google.com/go/compute/metadata"
	agentendpoint "cloud.google.com/go/osconfig/agentendpoint/apiv1"
	"github.com/GoogleCloudPlatform/osconfig/agentconfig"
	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/retryutil"
	"github.com/GoogleCloudPlatform/osconfig/tasker"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
)

const apiRetrySec = 600

var (
	errServerCancel      = errors.New("task canceled by server")
	errServiceNotEnabled = errors.New("service is not enabled for this project")
	errResourceExhausted = errors.New("ResourceExhausted")
	taskStateFile        = agentconfig.TaskStateFile()
	sameStateTimeWindow  = -5 * time.Second
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
		option.WithEndpoint(agentconfig.SvcEndpoint()),
		option.WithUserAgent(agentconfig.UserAgent()),
	}
	clog.Debugf(ctx, "Creating new agentendpoint client.")
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

// RegisterAgent calls RegisterAgent discarding the response.
func (c *Client) RegisterAgent(ctx context.Context) error {
	token, err := agentconfig.IDToken()
	if err != nil {
		return err
	}

	req := &agentendpointpb.RegisterAgentRequest{AgentVersion: agentconfig.Version(), SupportedCapabilities: agentconfig.Capabilities()}
	req.InstanceIdToken = "<redacted>"
	clog.DebugRPC(ctx, "RegisterAgent", req, nil)
	req.InstanceIdToken = token

	resp, err := c.raw.RegisterAgent(ctx, req)
	clog.DebugRPC(ctx, "RegisterAgent", nil, resp)

	return err
}

// reportInventory calls ReportInventory with the provided inventory.
func (c *Client) reportInventory(ctx context.Context, inventory *agentendpointpb.Inventory, reportFull bool) (*agentendpointpb.ReportInventoryResponse, error) {
	token, err := agentconfig.IDToken()
	if err != nil {
		return nil, err
	}

	hash := sha256.New()
	b, err := proto.Marshal(inventory)
	if err != nil {
		return nil, err
	}
	io.Copy(hash, bytes.NewReader(b))

	checksum := hex.EncodeToString(hash.Sum(nil))
	req := &agentendpointpb.ReportInventoryRequest{InventoryChecksum: checksum}
	if reportFull {
		req = &agentendpointpb.ReportInventoryRequest{InventoryChecksum: checksum, Inventory: inventory}
	}
	req.InstanceIdToken = "<redacted>"
	clog.DebugRPC(ctx, "ReportInventory", req, nil)
	req.InstanceIdToken = token

	resp, err := c.raw.ReportInventory(ctx, req)
	clog.DebugRPC(ctx, "ReportInventory", nil, resp)
	return resp, err
}

func (c *Client) startNextTask(ctx context.Context) (res *agentendpointpb.StartNextTaskResponse, err error) {
	token, err := agentconfig.IDToken()
	if err != nil {
		return nil, err
	}
	req := &agentendpointpb.StartNextTaskRequest{}
	req.InstanceIdToken = "<redacted>"
	clog.DebugRPC(ctx, "StartNextTask", req, nil)
	req.InstanceIdToken = token

	err = retryutil.RetryAPICall(ctx, apiRetrySec*time.Second, "StartNextTask", func() error {
		res, err = c.raw.StartNextTask(ctx, req)
		return err
	})
	clog.DebugRPC(ctx, "StartNextTask", nil, res)

	if err != nil {
		return nil, fmt.Errorf("error calling StartNextTask: %w", err)
	}
	return res, nil
}

func (c *Client) reportTaskProgress(ctx context.Context, req *agentendpointpb.ReportTaskProgressRequest) (res *agentendpointpb.ReportTaskProgressResponse, err error) {
	token, err := agentconfig.IDToken()
	if err != nil {
		return nil, err
	}
	req.InstanceIdToken = "<redacted>"
	clog.DebugRPC(ctx, "ReportTaskProgress", req, nil)
	req.InstanceIdToken = token

	err = retryutil.RetryAPICall(ctx, apiRetrySec*time.Second, "ReportTaskProgress", func() error {
		res, err = c.raw.ReportTaskProgress(ctx, req)
		return err
	})
	clog.DebugRPC(ctx, "ReportTaskProgress", nil, res)

	if err != nil {
		return nil, fmt.Errorf("error calling ReportTaskProgress: %w", err)
	}
	return res, nil
}

func (c *Client) reportTaskComplete(ctx context.Context, req *agentendpointpb.ReportTaskCompleteRequest) error {
	token, err := agentconfig.IDToken()
	if err != nil {
		return err
	}
	req.InstanceIdToken = "<redacted>"
	clog.DebugRPC(ctx, "ReportTaskComplete", req, nil)
	req.InstanceIdToken = token

	var res *agentendpointpb.ReportTaskCompleteResponse
	err = retryutil.RetryAPICall(ctx, apiRetrySec*time.Second, "ReportTaskComplete", func() error {
		res, err = c.raw.ReportTaskComplete(ctx, req)
		return err
	})
	clog.DebugRPC(ctx, "ReportTaskComplete", nil, res)

	if err != nil {
		return fmt.Errorf("error calling ReportTaskComplete: %w", err)
	}
	return nil
}

func (c *Client) runTask(ctx context.Context) {
	clog.Debugf(ctx, "Beginning run task loop.")
	for {
		res, err := c.startNextTask(ctx)
		if err != nil {
			clog.Errorf(ctx, "Error running StartNextTask, cannot continue: %v", err)
			return
		}

		task := res.GetTask()
		if task == nil {
			clog.Debugf(ctx, "No task to run, ending run task loop.")
			return
		}

		clog.Debugf(ctx, "Received task: %s.", task.GetTaskType())
		ctx = clog.WithLabels(ctx, map[string]string{"task_type": task.GetTaskType().String()})
		switch task.GetTaskType() {
		case agentendpointpb.TaskType_APPLY_PATCHES:
			if err := c.RunApplyPatches(ctx, task); err != nil {
				clog.Errorf(ctx, "Error running TaskType_APPLY_PATCHES: %v", err)
			}
		case agentendpointpb.TaskType_EXEC_STEP_TASK:
			if err := c.RunExecStep(ctx, task); err != nil {
				clog.Errorf(ctx, "Error running TaskType_EXEC_STEP_TASK: %v", err)
			}
		case agentendpointpb.TaskType_APPLY_CONFIG_TASK:
			if err := c.RunApplyConfig(ctx, task); err != nil {
				clog.Errorf(ctx, "Error running TaskType_APPLY_CONFIG_TASK: %v", err)
			}
		default:
			clog.Errorf(ctx, "Unknown task type: %v", task.GetTaskType())
		}
	}
}

func (c *Client) handleStream(ctx context.Context, stream agentendpointpb.AgentEndpointService_ReceiveTaskNotificationClient) error {
	for {
		clog.Debugf(ctx, "Waiting on ReceiveTaskNotification stream Recv().")
		if _, err := stream.Recv(); err != nil {
			// Return on any stream error, even a close, the caller will simply
			// reconnect the stream as needed.
			return err
		}
		clog.Debugf(ctx, "Received task notification.")

		// Only queue up one notifcation at a time. We should only ever
		// have one active task being worked on and one in the queue.
		select {
		case <-ctx.Done():
			// We have been canceled.
			return nil
		case c.noti <- struct{}{}:
			tasker.Enqueue(ctx, "TaskNotification", func() {
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
	token, err := agentconfig.IDToken()
	if err != nil {
		return nil, fmt.Errorf("error fetching Instance IDToken: %w", err)
	}

	req := &agentendpointpb.ReceiveTaskNotificationRequest{
		AgentVersion: agentconfig.Version(),
	}

	req.InstanceIdToken = "<redacted>"
	clog.DebugRPC(ctx, "ReceiveTaskNotification", req, nil)
	req.InstanceIdToken = token

	resp, err := c.raw.ReceiveTaskNotification(ctx, req)
	return resp, err
}

func (c *Client) loadTaskFromState(ctx context.Context) error {
	st, err := loadState(taskStateFile)
	if err != nil {
		return fmt.Errorf("loadState error: %w", err)
	}
	if st != nil && st.PatchTask != nil {
		st.PatchTask.client = c
		st.PatchTask.state = st
		tasker.Enqueue(ctx, "PatchRun", func() {
			st.PatchTask.run(ctx)
		})
	}

	return nil
}

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
		case codes.PermissionDenied:
			// Service is not enabled for this project.
			return errServiceNotEnabled
		case codes.ResourceExhausted:
			return errResourceExhausted
		}
	}
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
	clog.Debugf(ctx, "Running WaitForTaskNotification")
	ctx, c.cancel = context.WithCancel(ctx)

	clog.Debugf(ctx, "Checking local state file for saved task.")
	if err := c.loadTaskFromState(ctx); err != nil {
		clog.Errorf(ctx, err.Error())
	}

	clog.Debugf(ctx, "Setting up ReceiveTaskNotification stream watcher.")
	go func() {
		var resourceExhausted int
		var errs int
		var sleep time.Duration
		for {
			select {
			case <-ctx.Done():
				// We have been canceled.
				clog.Debugf(ctx, "Disabling WaitForTaskNotification")
				return
			default:
			}

			if err := c.waitForTask(ctx); err != nil {
				if errors.Is(err, errServiceNotEnabled) {
					// Service is disabled, close this client and return.
					clog.Warningf(ctx, "OSConfig Service is disabled.")
					c.Close()
					return
				}
				var ndr *metadata.NotDefinedError
				if errors.As(err, &ndr) {
					// No service account setup for this instance, close this client and return.
					clog.Warningf(ctx, "No service account set for instance.")
					c.Close()
					return
				}

				if errors.Is(err, errResourceExhausted) {
					resourceExhausted++
					sleep = retryutil.RetrySleep(resourceExhausted, 5)
				} else {
					// Retry any other errors with a modest backoff. Only retry up to 10
					// times, at that point return, the client will be recreated during the next
					// cycle.
					errs++
					clog.Warningf(ctx, "Error waiting for task (attempt %d of 10): %v", errs, err)
					resourceExhausted = 0
					if errs > 10 {
						c.Close()
						return
					}
					sleep = retryutil.RetrySleep(errs, 0)
				}
				time.Sleep(sleep)
				continue
			}
			errs = 0
		}
	}()
}
