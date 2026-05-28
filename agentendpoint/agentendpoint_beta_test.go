//  Copyright 2026 Google Inc. All Rights Reserved.
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
	"log"
	"net"
	"testing"
	"time"

	agentendpoint "cloud.google.com/go/osconfig/agentendpoint/apiv1beta"
	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

type agentEndpointServiceBetaTestServer struct {
	agentendpointpb.UnimplementedAgentEndpointServiceServer
	mockError error
}

func newAgentEndpointServiceBetaTestServer() *agentEndpointServiceBetaTestServer {
	return &agentEndpointServiceBetaTestServer{}
}

func (s *agentEndpointServiceBetaTestServer) LookupEffectiveGuestPolicy(ctx context.Context, req *agentendpointpb.LookupEffectiveGuestPolicyRequest) (*agentendpointpb.EffectiveGuestPolicy, error) {
	if s.mockError != nil {
		return nil, s.mockError
	}
	return &agentendpointpb.EffectiveGuestPolicy{}, nil
}

type testBetaClient struct {
	client *BetaClient
	server *grpc.Server
}

func (c *testBetaClient) close() {
	c.client.Close()
	c.server.Stop()
}

func newBetaTestClient(ctx context.Context, srv agentendpointpb.AgentEndpointServiceServer) (*testBetaClient, error) {
	listener := bufconn.Listen(bufSize)
	server := grpc.NewServer()
	agentendpointpb.RegisterAgentEndpointServiceServer(server, srv)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()

	var bufDialer = func(string, time.Duration) (net.Conn, error) {
		return listener.Dial()
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
		server: server,
	}, nil
}

func setupBetaClient(t *testing.T, ctx context.Context, mockErr error) *testBetaClient {
	t.Helper()
	srv := newAgentEndpointServiceBetaTestServer()
	srv.mockError = mockErr

	tc, err := newBetaTestClient(ctx, srv)
	if err != nil {
		t.Fatalf("failed to create beta test client: %v", err)
	}
	t.Cleanup(tc.close)
	return tc
}

func TestBetaClientClose(t *testing.T) {
	ctx := t.Context()
	tc, err := newBetaTestClient(ctx, newAgentEndpointServiceBetaTestServer())

	if err != nil {
		t.Fatalf("newBetaTestClient() error: %v", err)
	}
	defer tc.server.Stop()

	if tc.client.Closed() {
		t.Errorf("Closed() = true, want false")
	}

	if err := tc.client.Close(); err != nil {
		t.Errorf("Close() returned unexpected error: %v", err)
	}

	if !tc.client.Closed() {
		t.Errorf("Closed() = false, want true")
	}
}

func TestLookupEffectiveGuestPolicies(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name    string
		mockErr error
		wantErr error
	}{
		{
			name:    "successful server response, expect non-nil policy",
			mockErr: nil,
			wantErr: nil,
		},
		{
			name:    "server returns error, expect error",
			mockErr: errors.New("mock error"),
			wantErr: fmt.Errorf("error calling LookupEffectiveGuestPolicies: %w", errors.New(`code: "Unknown", message: "mock error", details: []`)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := setupBetaClient(t, ctx, tt.mockErr)
			gotPolicy, gotErr := tc.client.LookupEffectiveGuestPolicies(ctx)
			utiltest.AssertErrorMatchAndSkip(t, gotErr, tt.wantErr)
			if gotPolicy == nil {
				t.Fatal("LookupEffectiveGuestPolicies() returned nil policy, expect non-nil")
			}
		})
	}
}
