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
	"errors"
	"fmt"
	"testing"

	"cloud.google.com/go/osconfig/agentendpoint/apiv1beta/agentendpointpb"
	utilmocks "github.com/GoogleCloudPlatform/osconfig/util/mocks"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/golang/mock/gomock"
)

func TestBetaClientClose(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := utilmocks.NewMockAgentEndpointBetaClient(ctrl)
	client := &BetaClient{raw: mockClient, noti: make(chan struct{}, 1)}

	if client.Closed() {
		t.Errorf("Closed() = true, want false")
	}

	mockClient.EXPECT().Close().Return(nil)

	if err := client.Close(); err != nil {
		t.Errorf("Close() returned unexpected error: %v", err)
	}

	if !client.Closed() {
		t.Errorf("Closed() = false, want true")
	}
}

func TestLookupEffectiveGuestPolicies(t *testing.T) {
	ctx := t.Context()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockClient := utilmocks.NewMockAgentEndpointBetaClient(ctrl)
	client := &BetaClient{raw: mockClient, noti: make(chan struct{}, 1)}

	tests := []struct {
		name       string
		setup      func(*testing.T)
		wantPolicy *agentendpointpb.EffectiveGuestPolicy
		wantErr    error
	}{
		{
			name: "successful server response, expect non-nil policy",
			setup: func(t *testing.T) {
				mockClient.EXPECT().LookupEffectiveGuestPolicy(gomock.Any(), gomock.Any()).Return(&agentendpointpb.EffectiveGuestPolicy{}, nil)
			},
			wantPolicy: &agentendpointpb.EffectiveGuestPolicy{},
			wantErr:    nil,
		},
		{
			name: "server returns error, expect error",
			setup: func(t *testing.T) {
				mockClient.EXPECT().LookupEffectiveGuestPolicy(gomock.Any(), gomock.Any()).Return(nil, errors.New("mock error"))
			},
			wantErr: fmt.Errorf("error calling LookupEffectiveGuestPolicies: %w", errors.New("mock error")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(t)

			gotPolicy, gotErr := client.LookupEffectiveGuestPolicies(ctx)
			utiltest.AssertErrorMatch(t, gotErr, tt.wantErr)
			utiltest.AssertEquals(t, gotPolicy, tt.wantPolicy)
		})
	}
}
