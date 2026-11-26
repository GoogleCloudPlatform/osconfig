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

//go:build linux
// +build linux

package recipes

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func mkCharDevice(root *os.Root, name string, devMajor, devMinor uint32) error {
	return mknodInRoot(root, name, unix.S_IFCHR, int(unix.Mkdev(devMajor, devMinor)))
}

func mkBlockDevice(root *os.Root, name string, devMajor, devMinor uint32) error {
	return mknodInRoot(root, name, unix.S_IFBLK, int(unix.Mkdev(devMajor, devMinor)))
}

func mkFifo(root *os.Root, name string, mode uint32) error {
	return mknodInRoot(root, name, unix.S_IFIFO|mode, 0)
}

// mknodInRoot creates a filesystem node, device or fifo, by the path name relative to the root.
// os.Root has no method to create such nodes, so the parent directory is opened through the root,
// which ensures it is inside of the root, and the node is created relative to that directory.
func mknodInRoot(root *os.Root, name string, mode uint32, dev int) error {
	dir, err := root.Open(filepath.Dir(name))
	if err != nil {
		return err
	}
	defer dir.Close()

	return unix.Mknodat(int(dir.Fd()), filepath.Base(name), mode, dev)
}

func createDefaultEnvironment() ([]string, error) {
	return []string{}, nil
}
