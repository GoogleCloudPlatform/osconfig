//  Copyright 2026 Google Inc. All Rights Reserved.
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

//go:build windows
// +build windows

package agentendpoint

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
	"golang.org/x/sys/windows"
)

func TestRebootSystemWindows(t *testing.T) {
	ctx := t.Context()
	dllInitFailedLogoff := uint32(windows.STATUS_DLL_INIT_FAILED_LOGOFF)

	tests := []struct {
		name    string
		code    int
		hasCode bool
		cmdErr  error
		wantErr error
	}{
		{
			name:    "shutdown succeeds",
			code:    0,
			hasCode: false,
			cmdErr:  nil,
			wantErr: nil,
		},
		{
			name:    "ERROR_SHUTDOWN_IN_PROGRESS (1115) treated as non-fatal",
			code:    int(windows.ERROR_SHUTDOWN_IN_PROGRESS),
			hasCode: true,
			cmdErr:  fmt.Errorf("exit status %d", windows.ERROR_SHUTDOWN_IN_PROGRESS),
			wantErr: nil,
		},
		{
			name:    "ERROR_SHUTDOWN_IS_SCHEDULED (1190) treated as non-fatal",
			code:    int(windows.ERROR_SHUTDOWN_IS_SCHEDULED),
			hasCode: true,
			cmdErr:  fmt.Errorf("exit status %d", windows.ERROR_SHUTDOWN_IS_SCHEDULED),
			wantErr: nil,
		},
		{
			name:    "RPC_S_UNKNOWN_IF (1717) treated as non-fatal",
			code:    int(windows.RPC_S_UNKNOWN_IF),
			hasCode: true,
			cmdErr:  fmt.Errorf("exit status %d", windows.RPC_S_UNKNOWN_IF),
			wantErr: nil,
		},
		{
			name:    "STATUS_DLL_INIT_FAILED_LOGOFF (0xC000026B) treated as non-fatal",
			code:    int(dllInitFailedLogoff),
			hasCode: true,
			cmdErr:  fmt.Errorf("exit status %d", dllInitFailedLogoff),
			wantErr: nil,
		},
		{
			name:    "unexpected exit code returns error",
			code:    int(windows.ERROR_ACCESS_DENIED),
			hasCode: true,
			cmdErr:  errors.New("exit status 5"),
			wantErr: errors.New("exit status 5"),
		},
		{
			name:    "command launch error without exit code returns error",
			code:    0,
			hasCode: false,
			cmdErr:  errors.New("executable file not found"),
			wantErr: errors.New("executable file not found"),
		},
	}

	origRunShutdownCmd := runShutdownCmd
	t.Cleanup(func() { runShutdownCmd = origRunShutdownCmd })

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runShutdownCmd = func(_ context.Context, _ string) (int, bool, error) {
				return tt.code, tt.hasCode, tt.cmdErr
			}
			err := rebootSystem(ctx)
			utiltest.AssertErrorMatch(t, err, tt.wantErr)
		})
	}
}
