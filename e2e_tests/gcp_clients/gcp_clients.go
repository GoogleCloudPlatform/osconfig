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

package gcpclients

import (
	"context"
	"fmt"

	osconfigV1beta "cloud.google.com/go/osconfig/apiv1beta"
	"github.com/GoogleCloudPlatform/compute-image-tools/daisy/compute"
	"github.com/GoogleCloudPlatform/osconfig/e2e_tests/config"
	"google.golang.org/api/option"
)

var (
	computeClient        compute.Client
	osconfigClientV1beta *osconfigV1beta.Client
)

// PopulateClients populates the GCP clients.
func PopulateClients(ctx context.Context) error {
	if err := createComputeClient(ctx); err != nil {
		return err
	}
	return createOsConfigClientV1beta(ctx)
}

func createComputeClient(ctx context.Context) error {
	var err error
	computeClient, err = compute.NewClient(ctx, option.WithCredentialsFile(config.OauthPath()))
	return err
}

func createOsConfigClientV1beta(ctx context.Context) error {
	var err error
	osconfigClientV1beta, err = osconfigV1beta.NewClient(ctx, option.WithCredentialsFile(config.OauthPath()), option.WithEndpoint(config.SvcEndpoint()))
	return err
}

// GetComputeClient returns a singleton GCP client for osconfig tests
func GetComputeClient() (compute.Client, error) {
	if computeClient == nil {
		return nil, fmt.Errorf("compute client was not initialized")
	}
	return computeClient, nil
}

// GetOsConfigClientV1beta returns a singleton GCP client for osconfig tests
func GetOsConfigClientV1beta() (*osconfigV1beta.Client, error) {
	if osconfigClientV1beta == nil {
		return nil, fmt.Errorf("v1beta osconfig client was not initialized")
	}
	return osconfigClientV1beta, nil
}
