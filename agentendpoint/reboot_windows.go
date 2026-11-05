//  Copyright 2018 Google Inc. All Rights Reserved.
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
	"os"
	"os/exec"
	"path/filepath"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"golang.org/x/sys/windows"
)

var runShutdownCmd = func(ctx context.Context, root string) (int, bool, error) {
	err := exec.CommandContext(ctx, filepath.Join(root, `System32\shutdown.exe`), "/r", "/t", "00", "/f", "/d", "p:2:3").Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), true, err
	}
	return 0, false, err
}

// isShutdownAlreadyInProgress reports whether a shutdown.exe exit code
// indicates that a system shutdown or reboot is already scheduled or underway:
//   - ERROR_SHUTDOWN_IN_PROGRESS (1115): A system shutdown is in progress.
//   - ERROR_SHUTDOWN_IS_SCHEDULED (1190): A system shutdown has already been scheduled.
//   - RPC_S_UNKNOWN_IF (1717): wininit.exe has already unregistered the InitShutdown
//     RPC interface during an active system shutdown/preshutdown sequence.
//   - STATUS_DLL_INIT_FAILED_LOGOFF (0xC000026B): shutdown.exe failed to initialize
//     DLLs because the window station is already shutting down.
func isShutdownAlreadyInProgress(code int) bool {
	switch uint32(code) {
	case uint32(windows.ERROR_SHUTDOWN_IN_PROGRESS),
		uint32(windows.ERROR_SHUTDOWN_IS_SCHEDULED),
		uint32(windows.RPC_S_UNKNOWN_IF),
		uint32(windows.STATUS_DLL_INIT_FAILED_LOGOFF):
		return true
	default:
		return false
	}
}

func rebootSystem(ctx context.Context) error {
	ctx = context.WithoutCancel(ctx)
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	if code, hasCode, err := runShutdownCmd(ctx, root); err != nil {
		if hasCode && isShutdownAlreadyInProgress(code) {
			clog.Warningf(ctx, "System reboot/shutdown is already in progress (shutdown.exe exit status %d): %v", code, err)
			return nil
		}
		return err
	}
	return nil
}
