//  Copyright 2026 Google LLC
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

// Package gcp provides Compute Engine operations for E2E tests.
package gcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/e2e_tests_v2/internal/config"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"google.golang.org/api/osconfig/v1"
)

// VM represents an active or planned Compute Engine instance.
type VM struct {
	Project string
	Zone    string
	Name    string
	ID      uint64
}

// VMRequest contains settings needed to create a test VM.
type VMRequest struct {
	Project        string
	Zone           string
	Name           string
	Image          string
	MachineType    string
	Network        string
	Subnetwork     string
	ServiceAccount string
	Metadata       map[string]string
	Labels         map[string]string
}

// Client manages GCP API clients for Compute Engine and OS Config.
type Client struct {
	compute      *compute.Service
	osconfig     *osconfig.Service
	pollInterval time.Duration
}

// NewClient initializes the Compute Engine and OS Config API clients using Application Default Credentials.
func NewClient(ctx context.Context, cfg config.Config) (*Client, error) {
	var opts []option.ClientOption
	computeService, err := compute.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create compute service: %w", err)
	}
	osconfigService, err := osconfig.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create osconfig service: %w", err)
	}

	return &Client{
		compute:      computeService,
		osconfig:     osconfigService,
		pollInterval: cfg.PollInterval,
	}, nil
}

// CreateVM creates a Compute Engine instance and waits for the zonal insert operation to finish.
func (c *Client) CreateVM(ctx context.Context, req VMRequest) (*VM, error) {
	var metadata []*compute.MetadataItems
	for k, v := range req.Metadata {
		val := v
		metadata = append(metadata, &compute.MetadataItems{
			Key:   k,
			Value: &val,
		})
	}

	networkInterface := &compute.NetworkInterface{
		AccessConfigs: []*compute.AccessConfig{
			{
				Type: "ONE_TO_ONE_NAT",
				Name: "External NAT",
			},
		},
	}
	if req.Network != "" {
		networkInterface.Network = req.Network
	}
	if req.Subnetwork != "" {
		networkInterface.Subnetwork = req.Subnetwork
	}

	instance := &compute.Instance{
		Name:        req.Name,
		MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", req.Zone, req.MachineType),
		Labels:      req.Labels,
		Metadata:    &compute.Metadata{Items: metadata},
		NetworkInterfaces: []*compute.NetworkInterface{
			networkInterface,
		},
		Disks: []*compute.AttachedDisk{{
			AutoDelete: true,
			Boot:       true,
			InitializeParams: &compute.AttachedDiskInitializeParams{
				SourceImage: req.Image,
			},
		}},
		ServiceAccounts: []*compute.ServiceAccount{{
			Email:  req.ServiceAccount,
			Scopes: []string{compute.CloudPlatformScope},
		}},
	}

	op, err := c.compute.Instances.Insert(req.Project, req.Zone, instance).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("insert instance %s/%s/%s: %w", req.Project, req.Zone, req.Name, err)
	}

	if err := c.waitZoneOperation(ctx, req.Project, req.Zone, op.Name); err != nil {
		return nil, fmt.Errorf("create instance %s: %w", req.Name, err)
	}

	created, err := c.compute.Instances.Get(req.Project, req.Zone, req.Name).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get created instance %s: %w", req.Name, err)
	}

	return &VM{
		Project: req.Project,
		Zone:    req.Zone,
		Name:    req.Name,
		ID:      created.Id,
	}, nil
}

// DeleteVM idempotently deletes a VM and waits for completion.
func (c *Client) DeleteVM(ctx context.Context, project, zone, name string) error {
	op, err := c.compute.Instances.Delete(project, zone, name).Context(ctx).Do()
	if IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete instance %s/%s/%s: %w", project, zone, name, err)
	}
	if err := c.waitZoneOperation(ctx, project, zone, op.Name); err != nil {
		return fmt.Errorf("wait for instance %s deletion: %w", name, err)
	}
	return nil
}

// SerialOutput retrieves serial port 1 output from the VM.
func (c *Client) SerialOutput(ctx context.Context, project, zone, name string) (string, error) {
	output, err := c.compute.Instances.GetSerialPortOutput(project, zone, name).Port(1).Start(0).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("get serial output for %s: %w", name, err)
	}
	return output.Contents, nil
}

// GetInventory retrieves the OS Config inventory for an instance using the public OS Config API.
func (c *Client) GetInventory(ctx context.Context, project, zone, instance, view string) (*osconfig.Inventory, error) {
	name := fmt.Sprintf("projects/%s/locations/%s/instances/%s/inventory", project, zone, instance)
	call := c.osconfig.Projects.Locations.Instances.Inventories.Get(name).Context(ctx)
	if view != "" {
		call = call.View(view)
	}
	return call.Do()
}

func (c *Client) waitZoneOperation(ctx context.Context, project, zone, name string) error {
	var completed *compute.Operation
	err := PollUntil(ctx, c.pollInterval, fmt.Sprintf("zonal operation %s", name), func(ctx context.Context) (string, bool, error) {
		op, err := c.compute.ZoneOperations.Get(project, zone, name).Context(ctx).Do()
		if err != nil {
			if IsTransientComputeError(err) {
				return fmt.Sprintf("transient operation read: %v", err), false, nil
			}
			return "", false, err
		}
		if op.Status != "DONE" {
			return fmt.Sprintf("status=%s progress=%d", op.Status, op.Progress), false, nil
		}
		completed = op
		return "status=DONE", true, nil
	})
	if err != nil {
		return err
	}
	if completed.Error != nil && len(completed.Error.Errors) > 0 {
		var messages []string
		for _, item := range completed.Error.Errors {
			messages = append(messages, fmt.Sprintf("%s: %s", item.Code, item.Message))
		}
		return fmt.Errorf("operation %s failed: %s", name, strings.Join(messages, "; "))
	}
	return nil
}

// IsNotFound reports whether err is an HTTP 404 Not Found error.
func IsNotFound(err error) bool {
	return isHTTPStatus(err, http.StatusNotFound)
}

// IsTransientComputeError identifies retryable Compute Engine API errors.
func IsTransientComputeError(err error) bool {
	var apiErr *googleapi.Error
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.Code == http.StatusTooManyRequests || apiErr.Code >= 500
}

func isHTTPStatus(err error, code int) bool {
	var apiErr *googleapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == code
}
