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

// Package common contains common functions for use in the osconfig agent.
package common

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
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
	m := jsonpb.Marshaler{Indent: "  "}
	out, err := m.MarshalToString(pb)
	if err != nil {
		out = fmt.Sprintf("Error marshaling proto message: %v\n%s", err, out)
	}
	return out
}

// NormPath transforms a windows path into an extended-length path as described in
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#maxpath
// when not running on windows it will just return the input path. Copied from
// https://github.com/google/googet/blob/master/oswrap/oswrap_windows.go
func NormPath(path string) (string, error) {
	if runtime.GOOS != "windows" {
		return path, nil
	}

	if strings.HasPrefix(path, "\\\\?\\") {
		return path, nil
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	path = filepath.Clean(path)
	return "\\\\?\\" + path, nil
}

// Stubbed methods below
// this is done so that this function can be stubbed
// for unit testing

// Exists Checks if a file exists on the filesystem
var Exists = func(name string) bool {
	if _, err := os.Stat(name); os.IsNotExist(err) {
		return false
	}
	return true
}

// OsHostname is a wrapper to get os hostname
var OsHostname = func() (name string, err error) {
	return os.Hostname()
}

// Readfile is a wrapper to read file
var ReadFile = func(file string) ([]byte, error) {
	return ioutil.ReadFile(file)
}

// Run is a wrapper to execute terminal commands
var Run = func(cmd *exec.Cmd, logger *log.Logger) ([]byte, error) {
	logger.Printf("Running %q with args %q\n", cmd.Path, cmd.Args[1:])
	return cmd.CombinedOutput()
}
