//  Copyright 2020 Google Inc. All Rights Reserved.
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

package config

import (
	"context"
	"errors"
	"fmt"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/internal/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
)

// Resource is a single config resource.
type Resource struct {
	resource
	*agentendpointpb.ApplyConfigTask_Config_Resource

	managedResources *ManagedResources
	inDS             bool
}

// InDesiredState reports whether this resource is in the desired state.
// CheckState or EnforceState should be run prior to calling InDesiredState.
func (r *Resource) InDesiredState() bool {
	return r.inDS
}

// ManagedResources returns the resources that this resources manages.
func (r *Resource) ManagedResources() *ManagedResources {
	return r.managedResources
}

type resource interface {
	validate() (*ManagedResources, error)
	checkState() (bool, error)
	enforceState() (bool, error)
}

// ManagedResources are the resources that the resources manages.
type ManagedResources struct {
	Packages *Packages
}

// Validate validates this resource.
// Validate must be called before other methods.
func (r *Resource) Validate(ctx context.Context) error {
	switch x := r.GetResourceType().(type) {
	case *agentendpointpb.ApplyConfigTask_Config_Resource_Pkg:
		r.resource = resource(&packageResouce{ApplyConfigTask_Config_Resource_PackageResource: x.Pkg})
		/*
			case *agentendpointpb.ApplyConfigTask_Config_Resource_Repository:
			case *agentendpointpb.ApplyConfigTask_Config_Resource_Exec:
			case *agentendpointpb.ApplyConfigTask_Config_Resource_File_:
			case *agentendpointpb.ApplyConfigTask_Config_Resource_Archive:
			case *agentendpointpb.ApplyConfigTask_Config_Resource_Srvc:
		*/
	case nil:
		return errors.New("ResourceType field not set")
	default:
		return fmt.Errorf("ResourceType has unexpected type: %T", x)
	}

	var err error
	r.managedResources, err = r.validate()
	return err
}

// CheckState checks this resources state.
// Validate must be called prior to running CheckState.
func (r *Resource) CheckState(ctx context.Context) error {
	if r.resource == nil {
		return errors.New("CheckState run before Validate")
	}

	inDS, err := r.checkState()
	r.inDS = inDS
	return err
}

// EnforceState enforces this resources state.
// Validate must be called prior to running EnforceState.
func (r *Resource) EnforceState(ctx context.Context) error {
	if r.resource == nil {
		return errors.New("EnforceState run before Validate")
	}

	inDS, err := r.enforceState()
	r.inDS = inDS
	return err
}
