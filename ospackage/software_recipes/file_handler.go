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

package software_recipes

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"./oswrap"
)

// Wraps the file creation to allow a fake to be substituted for testing.
type OSFileHandler struct{}

func (fh *OSFileHandler) Create(path string) (io.WriteCloser, error) {
	return oswrap.Create(path)
}

func (fh *OSFileHandler) Open(path string) (io.ReadCloser, error) {
	return oswrap.Open(path)
}

type FileHandler interface {
	Create(string) (io.WriteCloser, error)
	Open(string) (io.ReadCloser, error)
}

type FakeFileHandler struct {
	CreatedFiles map[string]*StringWriteCloser
	FakeFiles    map[string]string
}

type StringReadCloser struct {
	*strings.Reader
	Closed bool
}

func (rc *StringReadCloser) Close() error {
	rc.Closed = true
	return nil
}

type StringWriteCloser struct {
	*bytes.Buffer
	Closed bool
}

func (wc *StringWriteCloser) Close() error {
	wc.Closed = true
	return nil
}

func (wc *StringWriteCloser) GetWrittenString() string {
	return wc.Buffer.String()
}

func (fh *FakeFileHandler) Create(path string) (io.WriteCloser, error) {
	wc := &StringWriteCloser{Buffer: &bytes.Buffer{}}
	if fh.CreatedFiles == nil {
		fh.CreatedFiles = make(map[string]*StringWriteCloser)
	}
	fh.CreatedFiles[path] = wc
	return wc, nil
}

func (fh *FakeFileHandler) Open(path string) (io.ReadCloser, error) {
	contents, ok := fh.FakeFiles[path]
	if !ok {
		return nil, fmt.Errorf("Fake contents for file with path %q has not been initialized", path)
	}
	return &StringReadCloser{Reader: strings.NewReader(contents)}, nil
}
