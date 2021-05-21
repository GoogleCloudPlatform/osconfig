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
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agentendpoint "cloud.google.com/go/osconfig/agentendpoint/apiv1"
	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"golang.org/x/oauth2/jws"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
)

var testIDToken string

func TestMain(m *testing.M) {
	cs := &jws.ClaimSet{
		Exp: time.Now().Add(1 * time.Hour).Unix(),
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		fmt.Printf("Error creating rsa key: %v", err)
		os.Exit(1)
	}
	testIDToken, err = jws.Encode(nil, cs, key)
	if err != nil {
		fmt.Printf("Error creating jwt token: %v", err)
		os.Exit(1)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, testIDToken)
	}))

	if err := os.Setenv("GCE_METADATA_HOST", strings.Trim(ts.URL, "http://")); err != nil {
		fmt.Printf("Error running os.Setenv: %v", err)
		os.Exit(1)
	}

	opts := logger.LogOpts{LoggerName: "OSConfigAgent", Debug: true, Writers: []io.Writer{os.Stdout}}
	logger.Init(context.Background(), opts)

	out := m.Run()
	ts.Close()
	os.Exit(out)
}

const bufSize = 1024 * 1024

type testClient struct {
	client *Client
	s      *grpc.Server
}

func (c *testClient) close() {
	c.client.Close()
	c.s.Stop()
}

func newTestClient(ctx context.Context, srv agentendpointpb.AgentEndpointServiceServer) (*testClient, error) {
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

	return &testClient{
		client: &Client{raw: client, noti: make(chan struct{}, 1)},
		s:      s,
	}, nil
}

type agentEndpointServiceTestServer struct {
	streamClose             chan struct{}
	streamSend              chan struct{}
	permissionError         chan struct{}
	taskStart               bool
	execTaskProgress        bool
	patchTaskProgress       bool
	applyConfigTaskProgress bool
	execTaskComplete        bool
	patchTaskComplete       bool
	applyConfigTaskComplete bool
	runTaskIDs              []string
}

func newAgentEndpointServiceTestServer() *agentEndpointServiceTestServer {
	return &agentEndpointServiceTestServer{
		streamClose:     make(chan struct{}, 1),
		streamSend:      make(chan struct{}, 1),
		permissionError: make(chan struct{}, 1),
	}
}

func (s *agentEndpointServiceTestServer) ReceiveTaskNotification(req *agentendpointpb.ReceiveTaskNotificationRequest, srv agentendpointpb.AgentEndpointService_ReceiveTaskNotificationServer) error {
	for {
		select {
		case <-s.streamClose:
			return nil
		case <-s.streamSend:
			srv.Send(&agentendpointpb.ReceiveTaskNotificationResponse{})
		case <-s.permissionError:
			return status.Errorf(codes.PermissionDenied, "")
		}
	}
}

func (s *agentEndpointServiceTestServer) StartNextTask(ctx context.Context, req *agentendpointpb.StartNextTaskRequest) (*agentendpointpb.StartNextTaskResponse, error) {
	// We first return an TaskType_EXEC_STEP_TASK, then TaskType_APPLY_PATCHES, then TaskType_APPLY_CONFIG_TASK.
	// After all tasks complete, we return nothing signalling the end to tasks.
	s.taskStart = true
	switch {
	case s.applyConfigTaskComplete && s.execTaskComplete && s.patchTaskComplete:
		return &agentendpointpb.StartNextTaskResponse{}, nil
	case !s.execTaskComplete:
		return &agentendpointpb.StartNextTaskResponse{Task: &agentendpointpb.Task{TaskType: agentendpointpb.TaskType_EXEC_STEP_TASK, TaskId: "TaskType_EXEC_STEP_TASK"}}, nil
	case !s.patchTaskComplete:
		return &agentendpointpb.StartNextTaskResponse{Task: &agentendpointpb.Task{TaskType: agentendpointpb.TaskType_APPLY_PATCHES, TaskId: "TaskType_APPLY_PATCHES"}}, nil
	case !s.applyConfigTaskComplete:
		return &agentendpointpb.StartNextTaskResponse{Task: &agentendpointpb.Task{TaskType: agentendpointpb.TaskType_APPLY_CONFIG_TASK, TaskId: "TaskType_APPLY_CONFIG_TASK"}}, nil
	default:
		return &agentendpointpb.StartNextTaskResponse{}, status.Errorf(codes.Unimplemented, "unexpected start next task")
	}
}

func (s *agentEndpointServiceTestServer) ReportTaskProgress(ctx context.Context, req *agentendpointpb.ReportTaskProgressRequest) (*agentendpointpb.ReportTaskProgressResponse, error) {
	// Simply record and send STOP.
	switch req.GetTaskType() {
	case agentendpointpb.TaskType_EXEC_STEP_TASK:
		s.execTaskProgress = true
	case agentendpointpb.TaskType_APPLY_PATCHES:
		s.patchTaskProgress = true
	case agentendpointpb.TaskType_APPLY_CONFIG_TASK:
		s.applyConfigTaskProgress = true
	default:
		return &agentendpointpb.ReportTaskProgressResponse{}, status.Errorf(codes.Unimplemented, "task type %q not implemented", req.GetTaskType())
	}
	return &agentendpointpb.ReportTaskProgressResponse{TaskDirective: agentendpointpb.TaskDirective_STOP}, nil
}

func (s *agentEndpointServiceTestServer) ReportTaskComplete(ctx context.Context, req *agentendpointpb.ReportTaskCompleteRequest) (*agentendpointpb.ReportTaskCompleteResponse, error) {
	// Record what task types we have seen, when the complete is called for TaskType_APPLY_CONFIG_TASK, close the stream.
	s.runTaskIDs = append(s.runTaskIDs, req.GetTaskId())
	switch req.GetTaskType() {
	case agentendpointpb.TaskType_EXEC_STEP_TASK:
		s.execTaskComplete = true
	case agentendpointpb.TaskType_APPLY_PATCHES:
		s.patchTaskComplete = true
	case agentendpointpb.TaskType_APPLY_CONFIG_TASK:
		s.applyConfigTaskComplete = true
	default:
		return &agentendpointpb.ReportTaskCompleteResponse{}, status.Errorf(codes.Unimplemented, "task type %q not implemented", req.GetTaskType())
	}
	if s.execTaskComplete && s.patchTaskComplete && s.applyConfigTaskComplete {
		s.streamClose <- struct{}{}
	}
	return &agentendpointpb.ReportTaskCompleteResponse{}, nil
}

func (*agentEndpointServiceTestServer) RegisterAgent(ctx context.Context, req *agentendpointpb.RegisterAgentRequest) (*agentendpointpb.RegisterAgentResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterAgent not implemented")
}

func (*agentEndpointServiceTestServer) ReportInventory(ctx context.Context, req *agentendpointpb.ReportInventoryRequest) (*agentendpointpb.ReportInventoryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReportInventory not implemented")
}

func TestWaitForTask(t *testing.T) {
	ctx := context.Background()
	srv := newAgentEndpointServiceTestServer()
	tc, err := newTestClient(ctx, srv)
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
	if !srv.execTaskProgress {
		t.Error("expected ReportTaskProgress for TaskType_EXEC_STEP_TASK to have been called")
	}
	if !srv.execTaskComplete {
		t.Error("expected ReportTaskComplete for TaskType_EXEC_STEP_TASK to have been called")
	}
	if !srv.patchTaskProgress {
		t.Error("expected ReportTaskProgress for TaskType_APPLY_PATCHES to have been called")
	}
	if !srv.patchTaskComplete {
		t.Error("expected ReportTaskComplete for TaskType_APPLY_PATCHES to have been called")
	}
	if !srv.applyConfigTaskProgress {
		t.Error("expected ReportTaskProgress for TaskType_APPLY_CONFIG_TASK to have been called")
	}
	if !srv.applyConfigTaskComplete {
		t.Error("expected ReportTaskComplete for TaskType_APPLY_CONFIG_TASK to have been called")
	}
}

func TestWaitForTaskErrors(t *testing.T) {
	ctx := context.Background()
	srv := newAgentEndpointServiceTestServer()
	tc, err := newTestClient(ctx, srv)
	if err != nil {
		t.Fatal(err)
	}

	// errServiceNotEnabled from PermissionDenied error.
	srv.permissionError <- struct{}{}
	if err := tc.client.waitForTask(ctx); !errors.Is(err, errServiceNotEnabled) {
		t.Errorf("did not get expected errServiceNotEnabled, got: %v", err)
	}

	// No error from a closed stream.
	srv.streamClose <- struct{}{}
	if err := tc.client.waitForTask(ctx); err != nil {
		t.Errorf("did not expect error from a closed stream: %v", err)
	}
}

func TestLoadPatchTaskFromState(t *testing.T) {
	ctx := context.Background()
	srv := newAgentEndpointServiceTestServer()
	tc, err := newTestClient(ctx, srv)
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

	srv.streamSend <- struct{}{}

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

	srv.execTaskComplete = true
	srv.applyConfigTaskComplete = true
	// Launch another patch task, this should run AFTER the task loaded from state file
	if err := tc.client.waitForTask(ctx); err != nil {
		t.Errorf("did not expect error from a closed stream: %v", err)
	}

	if srv.taskStart {
		t.Error("did not expect ReportTaskStart to have been called")
	}
	if !srv.patchTaskProgress {
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
