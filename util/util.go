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

// Package util contains common functions for use in the osconfig agent.
package util

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/GoogleCloudPlatform/osconfig/clog"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Logger holds log functions.
type Logger struct {
	Debugf   func(string, ...interface{})
	Infof    func(string, ...interface{})
	Warningf func(string, ...interface{})
	Errorf   func(string, ...interface{})
	Fatalf   func(string, ...interface{})
}

// PrettyFmt uses jsonpb to marshal a proto for pretty printing.
func PrettyFmt(pb proto.Message) string {
	m := &protojson.MarshalOptions{Indent: "  ", AllowPartial: true, UseProtoNames: true, EmitUnpopulated: true, UseEnumNumbers: false}
	return m.Format(pb)
}

// NormPath transforms a windows path into an extended-length path as described in
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#maxpath
// when not running on windows it will just return the input path.
func NormPath(path string) (string, error) {
	if strings.HasPrefix(path, `\\?\`) {
		return path, nil
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	if runtime.GOOS != "windows" {
		return path, nil
	}

	return `\\?\` + strings.ReplaceAll(path, "/", `\`), nil
}

// Exists check for the existence of a file
func Exists(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	if _, err := os.Stat(name); err != nil {
		return false
	}
	return true
}

// CommandRunner will execute the commands and return the results of that
// execution.
type CommandRunner interface {
	Run(ctx context.Context, command *exec.Cmd) ([]byte, []byte, error)
}

// DefaultRunner is a default CommandRunner.
type DefaultRunner struct{}

// Run takes precreated exec.Cmd and returns the stdout and stderr.
func (r *DefaultRunner) Run(ctx context.Context, cmd *exec.Cmd) ([]byte, []byte, error) {
	clog.Debugf(ctx, "Running %q with args %q\n", cmd.Path, cmd.Args[1:])
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	clog.Debugf(ctx, "%s %q output:\n%s", cmd.Path, cmd.Args[1:], strings.ReplaceAll(stdout.String(), "\n", "\n "))
	return stdout.Bytes(), stderr.Bytes(), err
}

// AtomicWrite attempts to atomically write a file.
func AtomicWrite(path string, content []byte, mode os.FileMode) (err error) {
	tmp, err := ioutil.TempFile(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return fmt.Errorf("unable to create temp file: %v", err)
	}

	tmpName := tmp.Name()
	// Make sure we cleanup on any errors.
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(content); err != nil {
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
