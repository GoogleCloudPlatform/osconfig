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

package config

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	agentendpointpb "github.com/GoogleCloudPlatform/osconfig/internal/google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1alpha1"
)

// Resource is a single config resource.
type Resource struct {
	inDS bool

	packageResource        *agentendpointpb.PackageResource
	repositoryResource     *agentendpointpb.RepositoryResource
	execResource           *agentendpointpb.ExecResource
	fileResource           *agentendpointpb.FileResource
	extractArchiveResource *agentendpointpb.ExtractArchiveResource
	serviceResource        *agentendpointpb.ServiceResource
}

// InDesiredState reports whether this resource is in the desired state.
// CheckState or EnforceState should be run prior to calling InDesiredState.
func (r *Resource) InDesiredState() bool {
	return r.inDS
}

// Unmarshal unmarshals this resource.
func (r *Resource) Unmarshal(ctx context.Context, res *agentendpointpb.ApplyConfigTask_Config_Resource) error {
	// TODO: implement

	switch protoreflect.Name(res.GetType().GetType()) {
	case r.packageResource.ProtoReflect().Descriptor().Name():
		b, err := proto.Marshal(res.GetPayload())
		if err != nil {
			return err
		}
		return protojson.Unmarshal(b, r.packageResource)
	case r.repositoryResource.ProtoReflect().Descriptor().Name():
	case r.execResource.ProtoReflect().Descriptor().Name():
	case r.fileResource.ProtoReflect().Descriptor().Name():
	case r.extractArchiveResource.ProtoReflect().Descriptor().Name():
	case r.serviceResource.ProtoReflect().Descriptor().Name():
	default:
		return fmt.Errorf("unknown resource type: %q", res.GetType().GetType())
	}
	return nil
}

// CheckState checks this resources state.
func (r *Resource) CheckState(ctx context.Context) error {
	// TODO: implement
	r.inDS = true
	return nil
}

// EnforceState enforces this resources state.
func (r *Resource) EnforceState(ctx context.Context) error {
	// TODO: implement
	r.inDS = true
	return nil
}
