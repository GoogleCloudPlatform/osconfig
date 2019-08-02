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

// Package common contains common functions for use in the osconfig agent.
package common

import (
	"fmt"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

// MarshalProto uses jsonpb to marshal a proto for pretty printing.
func MarshalProto(pb proto.Message) string {
	m := jsonpb.Marshaler{Indent: "  "}
	out, err := m.MarshalToString(pb)
	if err != nil {
		out = fmt.Sprintf("Error marshaling proto message: %v\n%s", err, out)
	}
	return out
}
