//  Copyright 2018 Google Inc. All Rights Reserved.
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

// Package compute contains wrappers around the GCE compute API.
package compute

import (
	"context"
	"fmt"
	"net/http"
	"path"
	"time"
	"os"

	daisyCompute "github.com/GoogleCloudPlatform/compute-image-tools/daisy/compute"
	computeApiBeta "google.golang.org/api/compute/v0.beta"
	computeApi "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

// Instance is a compute instance.
type Instance struct {
	*computeApi.Instance
	client        daisyCompute.Client
	Project, Zone string
}

// Cleanup deletes the Instance.
func (i *Instance) Cleanup() {
	if err := i.client.DeleteInstance(i.Project, i.Zone, i.Name); err != nil {
		fmt.Printf("Error deleting instance: %v\n", err)
	}
}

// WaitForGuestAttributes waits for guest attribute (queryPath, variableKey) to appear.
func (i *Instance) WaitForGuestAttributes(queryPath string, interval, timeout time.Duration) ([]*computeApiBeta.GuestAttributesEntry, error) {
	tick := time.Tick(interval)
	timedout := time.Tick(timeout)
	for {
		select {
		case <-timedout:
			return nil, fmt.Errorf("timed out waiting for guest attribute %q", queryPath)
		case <-tick:
			attr, err := i.GetGuestAttributes(queryPath)
			if err != nil {
				apiErr, ok := err.(*googleapi.Error)
				if ok && apiErr.Code == http.StatusNotFound {
					continue
				}
				return nil, err
			}
			return attr, nil
		}
	}
}

// GetGuestAttributes gets guest attributes for an instance.
func (i *Instance) GetGuestAttributes(queryPath string) ([]*computeApiBeta.GuestAttributesEntry, error) {
	resp, err := i.client.GetGuestAttributes(i.Project, i.Zone, i.Name, queryPath, "")
	if err != nil {
		return nil, err
	}
	if resp.QueryValue == nil {
		return nil, nil
	}

	return resp.QueryValue.Items, nil
}

// AddMetadata adds metadata to the instance.
func (i *Instance) AddMetadata(mdi ...*computeApi.MetadataItems) error {
	resp, err := i.client.GetInstance(i.Project, i.Zone, i.Name)
	if err != nil {
		return err
	}

	for _, old := range resp.Metadata.Items {
		found := false
		for _, new := range mdi {
			if old.Key == new.Key {
				found = true
				break
			}
		}
		if found {
			continue
		}
		mdi = append(mdi, old)
	}
	resp.Metadata.Items = mdi
	return i.client.SetInstanceMetadata(i.Project, i.Zone, i.Name, resp.Metadata)
}

// RecordSerialOutput stores the serial output of an instance to GCS bucket
func (i *Instance) RecordSerialOutput(ctx context.Context, logsPath string, port int64) {
	os.MkdirAll(logsPath, 0770)
	f, err := os.Create(path.Join(logsPath, fmt.Sprintf("%s-serial-port%d.log", i.Name, port)))
	if err != nil {
		fmt.Printf("Instance %q: error creating serial log file: %s", i.Name, err)
	}
	resp, err := i.client.GetSerialPortOutput(path.Base(i.Project), path.Base(i.Zone), i.Name, port, 0)
	if err != nil {
		// Instance is stopped or stopping.
		status, _ := i.client.InstanceStatus(path.Base(i.Project), path.Base(i.Zone), i.Name)
		if !isTerminal(status) {
			fmt.Printf("Instance %q: error getting serial port: %s", i.Name, err)
		}
		return
	}
	if _, err := f.Write([]byte(resp.Contents)); err != nil {
		fmt.Printf("Instance %q: error writing serial log file: %s", i.Name, err)
	}
	if err := f.Close(); err != nil {
		fmt.Printf("Instance %q: error closing serial log file: %s", i.Name, err)
	}
}

func isTerminal(status string) bool {
	return status == "TERMINATED" || status == "STOPPED" || status == "STOPPING"
}

// CreateInstance creates a compute instance.
func CreateInstance(client daisyCompute.Client, project, zone string, i *computeApi.Instance) (*Instance, error) {
	if err := client.CreateInstance(project, zone, i); err != nil {
		return nil, err
	}
	return &Instance{Instance: i, client: client, Project: project, Zone: zone}, nil
}

// BuildInstanceMetadataItem create an metadata item
func BuildInstanceMetadataItem(key, value string) *computeApi.MetadataItems {
	return &computeApi.MetadataItems{
		Key:   key,
		Value: func() *string { v := value; return &v }(),
	}
}
