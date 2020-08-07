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
	"fmt"
	"sync"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
)

// https://golang.org/pkg/context/#WithValue
type clogKey struct{}

var ctxValueKey = clogKey{}

type log struct {
	sync.Mutex
	labels map[string]string
	ctx    context.Context
}

func (l *log) log(msg string, sev logger.Severity) {
	// Set CallDepth 3, one for logger.Log, one for this function, and one for
	// the calling clog function.
	logger.Log(logger.LogEntry{Message: msg, Severity: sev, CallDepth: 3, Labels: l.labels})
}

// Debugf simulates logger.Debugf and adds context labels.
func Debugf(ctx context.Context, format string, args ...interface{}) {
	fromContext(ctx).log(fmt.Sprintf(format, args...), logger.Debug)
}

// Infof simulates logger.Infof and adds context labels.
func Infof(ctx context.Context, format string, args ...interface{}) {
	fromContext(ctx).log(fmt.Sprintf(format, args...), logger.Info)
}

// Warningf simulates logger.Warningf and context labels.
func Warningf(ctx context.Context, format string, args ...interface{}) {
	fromContext(ctx).log(fmt.Sprintf(format, args...), logger.Warning)
}

// Errorf simulates logger.Errorf and adds context labels.
func Errorf(ctx context.Context, format string, args ...interface{}) {
	fromContext(ctx).log(fmt.Sprintf(format, args...), logger.Error)
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
