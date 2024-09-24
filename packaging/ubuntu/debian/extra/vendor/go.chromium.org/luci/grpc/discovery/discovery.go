// Copyright 2016 The LUCI Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package discovery implements RPC service introspection.
package discovery

import (
	"context"
	"crypto/sha256"
	"fmt"
	"reflect"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.chromium.org/luci/grpc/prpc"
)

// New creates a discovery server for all given services.
//
// Service names have form "<pkg>.<service>", where "<pkg>" is name of the proto
// package and "<service>" is name of the service in the proto.
//
// The service descriptions must be registered already using
// RegisterDescriptorSetCompressed which is called by init() function
// generated by go.chromium.org/luci/grpc/cmd/cproto.
func New(serviceNames ...string) DiscoveryServer {
	return &service{exposed: func() []string { return serviceNames }}
}

// Enable registers a discovery service in the server.
//
// It makes all services registered in the server (now or later), including the
// discovery service itself, discoverable.
func Enable(server *prpc.Server) {
	RegisterDiscoveryServer(server, &service{exposed: server.ServiceNames})
}

type service struct {
	exposed func() []string // a dynamic list of services to expose

	m           sync.Mutex
	services    []string                      // services exposed in last Describe
	description *descriptor.FileDescriptorSet // their combined descriptor set
}

func (s *service) Describe(c context.Context, _ *Void) (*DescribeResponse, error) {
	services := s.exposed()

	s.m.Lock()
	defer s.m.Unlock()

	if !reflect.DeepEqual(services, s.services) {
		desc, err := combineDescriptors(services)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		s.description = desc
		s.services = append([]string(nil), services...)
	}

	return &DescribeResponse{
		Description: s.description,
		Services:    s.services,
	}, nil
}

// combineDescriptors creates one FileDescriptorSet that covers all services
// and their dependencies.
func combineDescriptors(serviceNames []string) (*descriptor.FileDescriptorSet, error) {
	result := &descriptor.FileDescriptorSet{}
	// seenFiles is a set of descriptor files keyed by SHA256 of their contents.
	seenFiles := map[[sha256.Size]byte]bool{}

	for _, s := range serviceNames {
		desc, err := GetDescriptorSet(s)
		if err != nil {
			return nil, fmt.Errorf("service %s: %s", s, err)
		}
		if desc == nil {
			return nil, fmt.Errorf(
				"descriptor for service %q is not found. "+
					"Did you compile it with go.chromium.org/luci/grpc/cmd/cproto?",
				s)
		}
		for _, f := range desc.GetFile() {
			binary, err := proto.Marshal(f)
			if err != nil {
				return nil, fmt.Errorf("could not marshal description of %s", f.GetName())
			}
			if hash := sha256.Sum256(binary); !seenFiles[hash] {
				result.File = append(result.File, f)
				seenFiles[hash] = true
			}
		}
	}
	return result, nil
}
