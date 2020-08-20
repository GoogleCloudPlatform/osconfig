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

package packages

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func ioctl(fd, req, arg uintptr) (err error) {
	_, _, e1 := unix.Syscall(unix.SYS_IOCTL, fd, req, arg)
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return
}

// This is used for anytime we need to parse YUM output.
// See https://bugzilla.redhat.com/show_bug.cgi?id=584525#c21
// TODO: We should probably look into a thin python shim we can
// interact with that the utilizes the yum libraries.
func runWithPty(cmd *exec.Cmd) ([]byte, []byte, error) {
	// Much of this logic was taken from, without the CGO stuff:
	// https://golang.org/src/os/signal/signal_cgo_test.go

	pty, err := os.OpenFile("/dev/ptmx", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	defer pty.Close()

	// grantpt doesn't appear to be required anymore.

	// unlockpt
	var i int
	if err := ioctl(pty.Fd(), unix.TIOCSPTLCK, uintptr(unsafe.Pointer(&i))); err != nil {
		return nil, nil, fmt.Errorf("error from ioctl TIOCSPTLCK: %v", err)
	}

	// ptsname
	var u uint32
	if err := ioctl(pty.Fd(), unix.TIOCGPTN, uintptr(unsafe.Pointer(&u))); err != nil {
		return nil, nil, fmt.Errorf("error from ioctl TIOCGPTN: %v", err)
	}
	path := filepath.Join("/dev/pts", strconv.Itoa(int(u)))

	tty, err := os.OpenFile(path, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	defer tty.Close()

	if err := unix.IoctlSetWinsize(int(pty.Fd()), syscall.TIOCSWINSZ, &unix.Winsize{Row: 1, Col: 500}); err != nil {
		return nil, nil, fmt.Errorf("error from IoctlSetWinsize: %v", err)
	}

	var stderr bytes.Buffer
	cmd.Stdin = tty
	cmd.Stdout = tty
	cmd.Stderr = &stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setctty: true,
		Setsid:  true,
		Ctty:    int(tty.Fd()),
	}

	var stdout bytes.Buffer
	var retErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		input := bufio.NewReader(pty)
		for {
			b, err := input.ReadBytes('\n')
			if err != nil {
				if perr, ok := err.(*os.PathError); ok {
					err = perr.Err
				}
				if err != io.EOF && err != syscall.EIO {
					retErr = err
				}
				return
			}

			if _, err := stdout.Write(b); err != nil {
				retErr = err
				return
			}
		}
	}()

	err = cmd.Run()
	if err := tty.Close(); err != nil {
		return nil, nil, err
	}

	wg.Wait()
	// Exit code 0 means no updates, 1 probably means there are but we just didn't install them.
	if err == nil {
		return nil, nil, err
	}
	return stdout.Bytes(), stderr.Bytes(), retErr
}
