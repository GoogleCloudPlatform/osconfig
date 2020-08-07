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

package clog

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWithLabels(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name   string
		labels map[string]string
		want   map[string]string
	}{
		{"NoLables", map[string]string{}, nil},
		{"OneLabel", map[string]string{"1": "1"}, map[string]string{"1": "1"}},
		{"AddFourLables", map[string]string{"2": "2", "3": "3", "4": "4", "5": "5"}, map[string]string{"1": "1", "2": "2", "3": "3", "4": "4", "5": "5"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx = WithLabels(ctx, tt.labels)
			got := fromContext(ctx).labels
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("Label mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
