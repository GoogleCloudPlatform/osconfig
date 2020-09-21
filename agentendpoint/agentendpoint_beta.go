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
	"sync"
	"time"

	agentendpoint "cloud.google.com/go/osconfig/agentendpoint/apiv1beta"
	"github.com/GoogleCloudPlatform/osconfig/agentconfig"
	"github.com/GoogleCloudPlatform/osconfig/clog"
	"github.com/GoogleCloudPlatform/osconfig/osinfo"
	"github.com/GoogleCloudPlatform/osconfig/retryutil"
	"github.com/GoogleCloudPlatform/osconfig/util"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

// BetaClient is a an agentendpoint client.
type BetaClient struct {
	raw    *agentendpoint.Client
	cancel context.CancelFunc
	noti   chan struct{}
	closed bool
	mx     sync.Mutex
}

// NewBetaClient a new agentendpoint Client.
func NewBetaClient(ctx context.Context) (*BetaClient, error) {
	opts := []option.ClientOption{
		option.WithoutAuthentication(), // Do not use oauth.
		option.WithGRPCDialOption(grpc.WithTransportCredentials(credentials.NewTLS(nil))), // Because we disabled Auth we need to specifically enable TLS.
		option.WithEndpoint(agentconfig.SvcEndpoint()),
	}
	clog.Debugf(ctx, "Creating new agentendpoint beta client.")
	c, err := agentendpoint.NewClient(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &BetaClient{raw: c, noti: make(chan struct{}, 1)}, nil
}

// Close cancels WaitForTaskNotification and closes the underlying ClientConn.
func (c *BetaClient) Close() error {
	// Lock so nothing can use the client while we are closing.
	c.mx.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.closed = true
	return c.raw.Close()
}

// Closed reports whether the Client has been closed.
func (c *BetaClient) Closed() bool {
	return c.closed
}

// LookupEffectiveGuestPolicies calls the agentendpoint service LookupEffectiveGuestPolicies.
func (c *BetaClient) LookupEffectiveGuestPolicies(ctx context.Context) (res *agentendpointpb.EffectiveGuestPolicy, err error) {
	info, err := osinfo.Get()
	if err != nil {
		return nil, err
	}

	req := &agentendpointpb.LookupEffectiveGuestPolicyRequest{
		OsShortName:    info.ShortName,
		OsVersion:      info.Version,
		OsArchitecture: info.Architecture,
	}

	token, err := agentconfig.IDToken()
	if err != nil {
		return nil, err
	}

	clog.Debugf(ctx, "Calling LookupEffectiveGuestPolicies with request:\n%s", util.PrettyFmt(req))
	req.InstanceIdToken = token

	// Only retry up to 30s for LookupEffectiveGuestPolicies in order to not hang up local configs.
	if err := retryutil.RetryAPICall(ctx, 30*time.Second, "LookupEffectiveGuestPolicies", func() error {
		res, err = c.raw.LookupEffectiveGuestPolicy(ctx, req)
		if err != nil {
			return err
		}
		clog.Debugf(ctx, "LookupEffectiveGuestPolicies response:\n%s", util.PrettyFmt(res))
		return nil
	}); err != nil {
		return nil, fmt.Errorf("error calling LookupEffectiveGuestPolicies: %w", err)
	}
	return res, nil
}
