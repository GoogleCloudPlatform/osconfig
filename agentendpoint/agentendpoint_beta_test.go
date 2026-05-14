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
	"log"
	"net"
	"testing"
	"time"

	agentendpoint "cloud.google.com/go/osconfig/agentendpoint/apiv1beta"
	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
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
	s      *grpc.Server
}

func newBetaTestClient(ctx context.Context, srv agentendpointpb.AgentEndpointServiceServer) (*testBetaClient, error) {
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

func TestBetaClientClose(t *testing.T) {
	ctx := context.Background()
	tc, err := newBetaTestClient(ctx, newAgentEndpointServiceBetaTestServer())

	if err != nil {
		t.Fatalf("newBetaTestClient() error: %v", err)
	}
	defer tc.s.Stop()

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
	ctx := context.Background()

	tests := []struct {
		name    string
		wantErr error
	}{
		{
			name:    "successful server response, want non-nil policy",
			wantErr: nil,
		},
		{
			name:    "server returns error, want error",
			wantErr: errors.New("mock error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newAgentEndpointServiceBetaTestServer()
			srv.mockError = tt.wantErr

			tc, err := newBetaTestClient(ctx, srv)
			if err != nil {
				t.Fatal(err)
			}
			defer tc.client.Close()
			defer tc.s.Stop()

			policy, err := tc.client.LookupEffectiveGuestPolicies(ctx)
			if (err != nil) != (tt.wantErr != nil) {
				t.Fatalf("LookupEffectiveGuestPolicies() unexpected error state: got err = %v, wantErr = %v", err, tt.wantErr)
			}

			if tt.wantErr == nil && policy == nil {
				t.Fatal("LookupEffectiveGuestPolicies() returned nil policy, want non-nil")
			}
		})
	}
}
