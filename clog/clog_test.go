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
	"fmt"
	"io"
	"testing"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestWithLabels(t *testing.T) {
	ctx := t.Context()

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

type testWriter struct {
	logs string
}

func (w *testWriter) Write(p []byte) (n int, err error) {
	w.logs = string(p)
	return len(p), nil
}

// Initializes logger and returns testWriter
func initTestLogger(ctx context.Context, t *testing.T) *testWriter {
	t.Helper()
	tw := &testWriter{}
	err := logger.Init(ctx, logger.LogOpts{
		LoggerName:          "test-logger",
		Writers:             []io.Writer{tw},
		DisableCloudLogging: true,
		DisableLocalLogging: true,
		Debug:               true,
		FormatFunction: func(e logger.LogEntry) string {
			return fmt.Sprintf("[%s] %s", e.Severity, e.Message)
		},
	})
	if err != nil {
		t.Fatalf("logger.Init error: %v", err)
	}
	return tw
}

func TestDebugRPC(t *testing.T) {
	utiltest.OverrideVariable(t, &DebugEnabled, true)
	ctx := t.Context()
	tw := initTestLogger(ctx, t)
	req := wrapperspb.String("req message")
	resp := wrapperspb.String("resp message")

	tests := []struct {
		name string
		req  proto.Message
		resp proto.Message
		want string
	}{
		{
			name: "ReqAndRespMethod",
			req:  req,
			resp: resp,
			want: `(?s)^\[Debug\] Called: ReqAndRespMethod with request:.*"req message".*\nresponse:\n.*"resp message".*`,
		},
		{
			name: "RespOnlyMethod",
			req:  nil,
			resp: resp,
			want: `(?s)^\[Debug\] Called: RespOnlyMethod with response:\n.*"resp message".*`,
		},
		{
			name: "ReqOnlyMethod",
			req:  req,
			resp: nil,
			want: `(?s)^\[Debug\] Calling: ReqOnlyMethod with request:\n.*"req message".*`,
		},
		{
			name: "NoReqNoRespMethod",
			req:  nil,
			resp: nil,
			want: `^$`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tw.logs = ""
			DebugRPC(ctx, tt.name, tt.req, tt.resp)
			utiltest.AssertFormatMatch(t, tw.logs, tt.want)
		})
	}
}

func TestLoggingFunctions(t *testing.T) {
	ctx := t.Context()
	tw := initTestLogger(ctx, t)
	tests := []struct {
		name    string
		logFunc func(ctx context.Context)
		want    string
	}{
		{
			name:    "Debugf",
			logFunc: func(ctx context.Context) { Debugf(ctx, "test debug %s", "msg") },
			want:    "[Debug] test debug msg\n",
		},
		{
			name:    "Infof",
			logFunc: func(ctx context.Context) { Infof(ctx, "test info %s", "msg") },
			want:    "[Info] test info msg\n",
		},
		{
			name:    "Warningf",
			logFunc: func(ctx context.Context) { Warningf(ctx, "test warn %s", "msg") },
			want:    "[Warning] test warn msg\n",
		},
		{
			name:    "Errorf",
			logFunc: func(ctx context.Context) { Errorf(ctx, "test error %s", "msg") },
			want:    "[Error] test error msg\n",
		},
		{
			name: "DebugStructured",
			logFunc: func(ctx context.Context) {
				DebugStructured(ctx, map[string]string{"key": "value"}, "test structured %s", "msg")
			},
			want: "[Debug] test structured msg\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tw.logs = ""
			tt.logFunc(ctx)
			utiltest.AssertEquals(t, tw.logs, tt.want)
		})
	}
}
