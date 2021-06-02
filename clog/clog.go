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

// Package clog is a Context logger.
package clog

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/pretty"
	"google.golang.org/protobuf/proto"
)

// DebugEnabled will log debug messages.
var DebugEnabled bool

// https://golang.org/pkg/context/#WithValue
type clogKey struct{}

var ctxValueKey = clogKey{}

type log struct {
	ctx    context.Context
	labels map[string]string
	sync.Mutex
}

func (l *log) log(structuredPayload interface{}, msg string, sev logger.Severity) {
	// Set CallDepth 3, one for logger.Log, one for this function, and one for
	// the calling clog function.
	logger.Log(logger.LogEntry{Message: msg, StructuredPayload: structuredPayload, Severity: sev, CallDepth: 3, Labels: l.labels})
}

// protoToJSON converts a proto message to a generic JSON object for the purpose
// of passing to Cloud Logging.
//
// Conversion errors are encoded in the JSON object rather than returned,
// because callers of logging functions should not be forced to handle errors.
func protoToJSON(p proto.Message) interface{} {
	bytes, err := pretty.MarshalOptions().Marshal(p)
	if err != nil {
		return fmt.Sprintf("Error converting proto: %s", err)
	}
	return json.RawMessage(bytes)
}

// DebugRPC logs a completed RPC call.
func DebugRPC(ctx context.Context, method string, req proto.Message, resp proto.Message) {
	// Do this here so we don't spend resources building the log message if we don't need to.
	if !DebugEnabled || (req == nil && resp == nil) {
		return
	}
	// The Cloud Logging library doesn't handle proto messages nor structures containing generic JSON.
	// To work around this we construct map[string]interface{} and fill it with JSON
	// resulting from explicit conversion of the proto messages.
	payload := map[string]interface{}{}
	payload["MethodName"] = method
	var msg string
	if resp != nil && req != nil {
		payload["Response"] = protoToJSON(resp)
		payload["Request"] = protoToJSON(req)
		msg = fmt.Sprintf("Called: %s with request:\n%s\nresponse:\n%s\n", method, pretty.Format(req), pretty.Format(resp))
	} else if resp != nil {
		payload["Response"] = protoToJSON(resp)
		msg = fmt.Sprintf("Called: %s with response:\n%s\n", method, pretty.Format(resp))
	} else {
		payload["Request"] = protoToJSON(req)
		msg = fmt.Sprintf("Calling: %s with request:\n%s\n", method, pretty.Format(req))
	}
	fromContext(ctx).log(payload, msg, logger.Debug)
}

// DebugStructured is like Debugf but sends structuredPayload instead of the text message
// to Cloud Logging.
func DebugStructured(ctx context.Context, structuredPayload interface{}, format string, args ...interface{}) {
	fromContext(ctx).log(structuredPayload, fmt.Sprintf(format, args...), logger.Debug)
}

// Debugf simulates logger.Debugf and adds context labels.
func Debugf(ctx context.Context, format string, args ...interface{}) {
	fromContext(ctx).log(nil, fmt.Sprintf(format, args...), logger.Debug)
}

// Infof simulates logger.Infof and adds context labels.
func Infof(ctx context.Context, format string, args ...interface{}) {
	fromContext(ctx).log(nil, fmt.Sprintf(format, args...), logger.Info)
}

// Warningf simulates logger.Warningf and context labels.
func Warningf(ctx context.Context, format string, args ...interface{}) {
	fromContext(ctx).log(nil, fmt.Sprintf(format, args...), logger.Warning)
}

// Errorf simulates logger.Errorf and adds context labels.
func Errorf(ctx context.Context, format string, args ...interface{}) {
	fromContext(ctx).log(nil, fmt.Sprintf(format, args...), logger.Error)
}

func (l *log) clone() *log {
	l.Lock()
	defer l.Unlock()

	labels := map[string]string{}
	for k, v := range l.labels {
		labels[k] = v
	}

	return &log{
		labels: labels,
	}
}

func forContext(ctx context.Context) (*log, context.Context) {
	cv := ctx.Value(ctxValueKey)
	l, ok := cv.(*log)
	if !ok {
		l = &log{labels: map[string]string{}}
	} else {
		l = l.clone()
	}

	ctx = context.WithValue(ctx, ctxValueKey, l)
	l.ctx = ctx
	return l, ctx
}

func fromContext(ctx context.Context) *log {
	if ctx == nil {
		return &log{}
	}

	v := ctx.Value(ctxValueKey)
	l, ok := v.(*log)
	if !ok {
		l = &log{}
	}
	return l
}

// WithLabels makes a log and context and adds the labels (overwriting any with the same key).
func WithLabels(ctx context.Context, labels map[string]string) context.Context {
	if len(labels) == 0 {
		return ctx
	}

	l, ctx := forContext(ctx)

	l.Lock()
	defer l.Unlock()

	for k, v := range labels {
		l.labels[k] = v
	}

	return ctx
}
