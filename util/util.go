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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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

// WriteFile writes data from the provided reader to the localPath checking the checksum if provided.
func WriteFile(r io.Reader, checksum, localPath string, mode os.FileMode) (string, error) {
	localPath, err := NormPath(localPath)
	if err != nil {
		return "", err
	}
	file, err := os.OpenFile(localPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err = io.Copy(io.MultiWriter(file, hasher), r); err != nil {
		return "", err
	}
	computed := hex.EncodeToString(hasher.Sum(nil))
	if checksum != "" && !strings.EqualFold(checksum, computed) {
		return "", fmt.Errorf("got %q for checksum, expected %q", computed, checksum)
	}
	return computed, nil
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
