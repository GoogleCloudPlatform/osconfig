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
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentendpoint "cloud.google.com/go/osconfig/agentendpoint/apiv1beta"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

type testBetaClient struct {
	client *BetaClient
	s      *grpc.Server
}

func (c *testBetaClient) close() {
	c.client.Close()
	c.s.Stop()
}

func newTestBetaClient(ctx context.Context, srv agentendpointpb.AgentEndpointServiceServer) (*testBetaClient, error) {
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	agentendpointpb.RegisterAgentEndpointServiceServer(s, srv)

	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()

	var bufDialer = func(string, time.Duration) (net.Conn, error) {
		return lis.Dial()
	}

	conn, err := grpc.DialContext(ctx, "bufnet", grpc.WithDialer(bufDialer), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	client, err := agentendpoint.NewClient(ctx, option.WithGRPCConn(conn))
	if err != nil {
		return nil, err
	}

	return &testBetaClient{
		client: &BetaClient{raw: client, noti: make(chan struct{}, 1)},
		s:      s,
	}, nil
}

type agentEndpointServiceBetaTestServer struct {
	streamClose       chan struct{}
	unavailableError  chan struct{}
	streamSend        chan struct{}
	permissionError   chan struct{}
	taskStart         bool
	execTaskStart     bool
	patchTaskStart    bool
	execTaskComplete  bool
	patchTaskComplete bool
	runTaskIDs        []string
}

func newAgentEndpointServiceBetaTestServer() *agentEndpointServiceBetaTestServer {
	return &agentEndpointServiceBetaTestServer{
		streamClose:      make(chan struct{}, 1),
		unavailableError: make(chan struct{}, 1),
		streamSend:       make(chan struct{}, 1),
		permissionError:  make(chan struct{}, 1),
	}
}

func (s *agentEndpointServiceBetaTestServer) ReceiveTaskNotification(req *agentendpointpb.ReceiveTaskNotificationRequest, srv agentendpointpb.AgentEndpointService_ReceiveTaskNotificationServer) error {
	for {
		select {
		case <-s.streamClose:
			return nil
		case <-s.streamSend:
			srv.Send(&agentendpointpb.ReceiveTaskNotificationResponse{})
		case <-s.unavailableError:
			return status.Errorf(codes.Unavailable, "")
		case <-s.permissionError:
			return status.Errorf(codes.PermissionDenied, "")
		}
	}
}
func (s *agentEndpointServiceBetaTestServer) StartNextTask(ctx context.Context, req *agentendpointpb.StartNextTaskRequest) (*agentendpointpb.StartNextTaskResponse, error) {
	// We first return an TaskType_EXEC_STEP_TASK, then TaskType_APPLY_PATCHES. If patchTaskRun we return nothing signalling the end to tasks.
	switch {
	case s.patchTaskComplete:
		return &agentendpointpb.StartNextTaskResponse{}, nil
	case s.taskStart:
		return &agentendpointpb.StartNextTaskResponse{Task: &agentendpointpb.Task{TaskType: agentendpointpb.TaskType_APPLY_PATCHES, TaskId: "TaskType_APPLY_PATCHES"}}, nil
	default:
		s.taskStart = true
		return &agentendpointpb.StartNextTaskResponse{Task: &agentendpointpb.Task{TaskType: agentendpointpb.TaskType_EXEC_STEP_TASK, TaskId: "TaskType_EXEC_STEP_TASK"}}, nil
	}
}
func (s *agentEndpointServiceBetaTestServer) ReportTaskProgress(ctx context.Context, req *agentendpointpb.ReportTaskProgressRequest) (*agentendpointpb.ReportTaskProgressResponse, error) {
	// Simply record and send STOP.
	if req.GetTaskType() == agentendpointpb.TaskType_EXEC_STEP_TASK {
		s.execTaskStart = true
	}
	if req.GetTaskType() == agentendpointpb.TaskType_APPLY_PATCHES {
		s.patchTaskStart = true
	}
	return &agentendpointpb.ReportTaskProgressResponse{TaskDirective: agentendpointpb.TaskDirective_STOP}, nil
}
func (s *agentEndpointServiceBetaTestServer) ReportTaskComplete(ctx context.Context, req *agentendpointpb.ReportTaskCompleteRequest) (*agentendpointpb.ReportTaskCompleteResponse, error) {
	// Record what task types we have seen, when the complete is called for TaskType_APPLY_PATCHES, close the stream.
	s.runTaskIDs = append(s.runTaskIDs, req.GetTaskId())
	if req.GetTaskType() == agentendpointpb.TaskType_EXEC_STEP_TASK {
		s.execTaskComplete = true
	}
	if req.GetTaskType() == agentendpointpb.TaskType_APPLY_PATCHES {
		s.patchTaskComplete = true
		s.streamClose <- struct{}{}
	}
	return &agentendpointpb.ReportTaskCompleteResponse{}, nil
}
func (*agentEndpointServiceBetaTestServer) LookupEffectiveGuestPolicy(ctx context.Context, req *agentendpointpb.LookupEffectiveGuestPolicyRequest) (*agentendpointpb.EffectiveGuestPolicy, error) {
	return nil, status.Errorf(codes.Unimplemented, "method LookupEffectiveGuestPolicies not implemented")
}

func (*agentEndpointServiceBetaTestServer) RegisterAgent(ctx context.Context, req *agentendpointpb.RegisterAgentRequest) (*agentendpointpb.RegisterAgentResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterAgent not implemented")
}

func TestWaitForTaskBeta(t *testing.T) {
	ctx := context.Background()
	srv := newAgentEndpointServiceBetaTestServer()
	tc, err := newTestBetaClient(ctx, srv)
	if err != nil {
		t.Fatal(err)
	}
	defer tc.close()

	td, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)
	taskStateFile = filepath.Join(td, "testState")

	// Stream recieve.
	srv.streamSend <- struct{}{}
	if err := tc.client.waitForTask(ctx); err != nil {
		t.Errorf("did not expect error from a closed stream: %v", err)
	}
	if !srv.taskStart {
		t.Error("expected ReportTaskStart to have been called")
	}
	if !srv.execTaskStart {
		t.Error("expected ReportTaskProgress for TaskType_EXEC_STEP_TASK to have been called")
	}
	if !srv.execTaskComplete {
		t.Error("expected ReportTaskComplete for TaskType_EXEC_STEP_TASK to have been called")
	}
	if !srv.patchTaskStart {
		t.Error("expected ReportTaskProgress for TaskType_APPLY_PATCHES to have been called")
	}
	if !srv.patchTaskComplete {
		t.Error("expected ReportTaskComplete for TaskType_APPLY_PATCHES to have been called")
	}
}

func TestWaitForTaskErrorsBeta(t *testing.T) {
	ctx := context.Background()
	srv := newAgentEndpointServiceBetaTestServer()
	tc, err := newTestBetaClient(ctx, srv)
	if err != nil {
		t.Fatal(err)
	}

	// No error from Unavailable error.
	srv.unavailableError <- struct{}{}
	if err := tc.client.waitForTask(ctx); err != nil {
		t.Errorf("did not expect error from a server error: %v", err)
	}

	// errServiceNotEnabled from PermissionDenied error.
	srv.permissionError <- struct{}{}
	if err := tc.client.waitForTask(ctx); err != errServiceNotEnabled {
		t.Errorf("did not get expected errServiceNotEnabled, got: %v", err)
	}

	// No error from a closed stream.
	srv.streamClose <- struct{}{}
	if err := tc.client.waitForTask(ctx); err != nil {
		t.Errorf("did not expect error from a closed stream: %v", err)
	}

	tc.close()
	// Error from a closed server
	if err := tc.client.waitForTask(ctx); err == nil {
		t.Error("expected error from a closed server")
	}
}

func TestLoadTaskFromStateBeta(t *testing.T) {
	ctx := context.Background()
	srv := newAgentEndpointServiceBetaTestServer()
	tc, err := newTestBetaClient(ctx, srv)
	if err != nil {
		t.Fatal(err)
	}
	defer tc.close()

	td, err := ioutil.TempDir(os.TempDir(), "")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	defer os.RemoveAll(td)
	taskStateFile = filepath.Join(td, "testState")

	// No state.
	if err := tc.client.loadTaskFromState(ctx); err != nil {
		t.Error(err)
	}
	if srv.taskStart {
		t.Error("expected ReportTaskStart to not have been called")
	}

	// Bad state.
	if err := ioutil.WriteFile(taskStateFile, []byte("bad"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := tc.client.loadTaskFromState(ctx); err == nil {
		t.Error("expected error from loadTaskFromState")
	}

	// Existing task.
	taskID := "foo"
	if err := ioutil.WriteFile(taskStateFile, []byte(fmt.Sprintf(`{"PatchTask":{"TaskID":"%s", "PatchStep": "%s"}}`, taskID, patching)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := tc.client.loadTaskFromState(ctx); err != nil {
		t.Fatal(err)
	}

	// Launch another, this should run AFTER the task loaded from state file, but the previous task should have closed the stream.
	if err := tc.client.waitForTask(ctx); err != nil {
		t.Errorf("did not expect error from a closed stream: %v", err)
	}

	if srv.taskStart {
		t.Error("did not expect ReportTaskStart to have been called")
	}
	if !srv.patchTaskStart {
		t.Error("expected ReportTaskProgress for TaskType_APPLY_PATCHES to have been called")
	}
	if !srv.patchTaskComplete {
		t.Error("expected ReportTaskComplete for TaskType_APPLY_PATCHES to have been called")
	}
	if len(srv.runTaskIDs) != 1 {
		t.Fatalf("expected srv.runTaskIDs to have a length of 1, not %d", len(srv.runTaskIDs))
	}
	if srv.runTaskIDs[0] != taskID {
		t.Errorf("first entry in runTaskIDs does not match taskID, %q, %q", srv.runTaskIDs, taskID)
	}
}
