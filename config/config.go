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
	"runtime"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1"
)

var goos = runtime.GOOS

// OSPolicyResource is a single OSPolicy resource.
type OSPolicyResource struct {
	resource
	*agentendpointpb.OSPolicy_Resource

	managedResources *ManagedResources
	inDesiredState   bool
}

// InDesiredState reports whether this resource is in the desired state.
// CheckState or EnforceState should be run prior to calling InDesiredState.
func (r *OSPolicyResource) InDesiredState() bool {
	return r.inDesiredState
}

// ManagedResources returns the resources that this OSPolicyResource manages.
func (r *OSPolicyResource) ManagedResources() *ManagedResources {
	return r.managedResources
}

type resource interface {
	validate(context.Context) (*ManagedResources, error)
	checkState(context.Context) (bool, error)
	enforceState(context.Context) (bool, error)
	populateOutput(*agentendpointpb.OSPolicyResourceCompliance)
	cleanup(context.Context) error
}

// ManagedResources are the resources that an OSPolicyResource manages.
type ManagedResources struct {
	Packages     []ManagedPackage
	Repositories []ManagedRepository
	Files        []ManagedFile
}

// Validate validates this resource.
// Validate must be called before other methods.
func (r *OSPolicyResource) Validate(ctx context.Context) error {
	switch x := r.GetResourceType().(type) {
	case *agentendpointpb.OSPolicy_Resource_Pkg:
		r.resource = resource(&packageResouce{OSPolicy_Resource_PackageResource: x.Pkg})
	case *agentendpointpb.OSPolicy_Resource_Repository:
		r.resource = resource(&repositoryResource{OSPolicy_Resource_RepositoryResource: x.Repository})
	case *agentendpointpb.OSPolicy_Resource_File_:
		r.resource = resource(&fileResource{OSPolicy_Resource_FileResource: x.File})
	case *agentendpointpb.OSPolicy_Resource_Exec:
		r.resource = resource(&execResource{OSPolicy_Resource_ExecResource: x.Exec})

	case nil:
		return errors.New("ResourceType field not set")
	default:
		return fmt.Errorf("ResourceType has unexpected type: %T", x)
	}

	var err error
	r.managedResources, err = r.validate(ctx)
	return err
}

// CheckState checks this resources state.
// Validate must be called prior to running CheckState.
func (r *OSPolicyResource) CheckState(ctx context.Context) error {
	if r.resource == nil {
		return errors.New("CheckState run before Validate")
	}

	inDesiredState, err := r.checkState(ctx)
	r.inDesiredState = inDesiredState
	return err
}

// EnforceState enforces this resources state.
// Validate must be called prior to running EnforceState.
func (r *OSPolicyResource) EnforceState(ctx context.Context) error {
	if r.resource == nil {
		return errors.New("EnforceState run before Validate")
	}

	inDesiredState, err := r.enforceState(ctx)
	r.inDesiredState = inDesiredState
	return err
}

// PopulateOutput populates the output field of the provided
// OSPolicyResourceCompliance for this resource.
func (r *OSPolicyResource) PopulateOutput(rCompliance *agentendpointpb.OSPolicyResourceCompliance) error {
	if r.resource == nil {
		return errors.New("PopulateOutput run before Validate")
	}
	r.populateOutput(rCompliance)
	return nil
}

// Cleanup cleans up any temporary files that this resource may have created.
func (r *OSPolicyResource) Cleanup(ctx context.Context) error {
	if r.resource == nil {
		return errors.New("Cleanup run before Validate")
	}
	return r.cleanup(ctx)
}
