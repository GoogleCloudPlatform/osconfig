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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"
	"github.com/GoogleCloudPlatform/osconfig/packages"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

var (
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const (
	serviceName = "google_osconfig_agent"

	// https://docs.microsoft.com/en-us/windows/desktop/api/fileapi/nf-fileapi-lockfileex
	LOCKFILE_EXCLUSIVE_LOCK   = 2
	LOCKFILE_FAIL_IMMEDIATELY = 1
)

func lockFileEx(hFile uintptr, dwFlags, nNumberOfBytesToLockLow, nNumberOfBytesToLockHigh uint32, lpOverlapped *syscall.Overlapped) (err error) {
	ret, _, _ := procLockFileEx.Call(
		hFile,
		uintptr(dwFlags),
		0,
		uintptr(nNumberOfBytesToLockLow),
		uintptr(nNumberOfBytesToLockHigh),
		uintptr(unsafe.Pointer(lpOverlapped)),
	)
	// If the function succeeds, the return value is nonzero.
	if ret == 0 {
		return errors.New("LockFileEx unable to obtain lock")
	}
	return nil
}

func unlockFileEx(hFile uintptr, nNumberOfBytesToLockLow, nNumberOfBytesToLockHigh uint32, lpOverlapped *syscall.Overlapped) (err error) {
	ret, _, _ := procUnlockFileEx.Call(
		hFile,
		0,
		uintptr(nNumberOfBytesToLockLow),
		uintptr(nNumberOfBytesToLockHigh),
		uintptr(unsafe.Pointer(lpOverlapped)),
	)
	// If the function succeeds, the return value is nonzero.
	if ret == 0 {
		return errors.New("UnlockFileEx unable to unlock")
	}
	return nil
}

func obtainLock() {
	lockFile := `C:\Program Files\Google\OSConfig\lock`

	err := os.MkdirAll(filepath.Dir(lockFile), 0755)
	if err != nil && !os.IsExist(err) {
		logger.Fatalf("Cannot obtain agent lock: %v", err)
	}
	f, err := os.OpenFile(lockFile, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil && !os.IsExist(err) {
		logger.Fatalf("Cannot obtain agent lock: %v", err)
	}

	if err := lockFileEx(f.Fd(), LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY, 1, 0, &syscall.Overlapped{}); err != nil {
		logger.Fatalf("OSConfig agent lock already held, is the agent already running?")
	}

	deferredFuncs = append(deferredFuncs, func() { unlockFileEx(f.Fd(), 1, 0, &syscall.Overlapped{}); f.Close(); os.Remove(lockFile) })
}

type service struct {
	ctx context.Context
	run func(context.Context)
}

func (s *service) Execute(_ []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}
	ctx, cncl := context.WithCancel(s.ctx)
	defer cncl()
	done := make(chan struct{})

	go func() {
		s.run(ctx)
		close(done)
	}()
	status <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for {
		select {
		case <-done:
			status <- svc.Status{State: svc.StopPending}
			return false, 0
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				status <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				cncl()
			default:
			}
		}
	}
}

func runService(ctx context.Context) {
	if err := svc.Run(serviceName, &service{run: run, ctx: ctx}); err != nil {
		logger.Fatalf("svc.Run error: %v", err)
	}
}

func wuaUpdates(query string) error {
	updts, err := packages.WUAUpdates(query)
	if err != nil {
		return err
	}
	data, err := json.Marshal(updts)
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stdout, string(data))
	return nil
}
