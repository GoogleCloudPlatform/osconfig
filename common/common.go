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
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
)

var (
	testStorageClient *storage.Client
	testHTTPClient    *http.Client
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

// FetchWithGCS produces a GCS reader for a GCS object.
func FetchWithGCS(ctx context.Context, bucket, object string, generation int64) (*storage.Reader, error) {
	client, err := newStorageClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	oh := client.Bucket(bucket).Object(object)
	if generation != 0 {
		oh = oh.Generation(generation)
	}

	r, err := oh.NewReader(ctx)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// FetchWithHTTP retrieves a HTTP response from a Get URI.
func FetchWithHTTP(ctx context.Context, uri string) (*http.Response, error) {
	resp, err := newHTTPClient().Get(uri)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got http status %d when attempting to download artifact", resp.StatusCode)
	}

	return resp, nil
}

// DownloadStream writes the artifact to a local path.
func DownloadStream(r io.Reader, checksum string, localPath string) error {
	localPath, err := NormPath(localPath)
	if err != nil {
		return err
	}
	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err = io.Copy(io.MultiWriter(file, hasher), r); err != nil {
		return err
	}
	computed := hex.EncodeToString(hasher.Sum(nil))
	if checksum != "" && !strings.EqualFold(checksum, computed) {
		return fmt.Errorf("got %q for checksum, expected %q", computed, checksum)
	}
	return nil
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

func newStorageClient(ctx context.Context) (*storage.Client, error) {
	if testStorageClient != nil {
		return testStorageClient, nil
	}
	return storage.NewClient(ctx)
}

func newHTTPClient() *http.Client {
	if testHTTPClient != nil {
		return testHTTPClient
	}
	return &http.Client{}
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

// ReadFile is a wrapper to read file
var ReadFile = func(file string) ([]byte, error) {
	return ioutil.ReadFile(file)
}

// Run is a wrapper to execute terminal commands
var Run = func(cmd *exec.Cmd, logger *log.Logger) ([]byte, error) {
	logger.Printf("Running %q with args %q\n", cmd.Path, cmd.Args[1:])
	return cmd.CombinedOutput()
}
