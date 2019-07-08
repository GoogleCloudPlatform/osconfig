//+build windows

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

package recipes

import (
	"os"
	"path/filepath"
	"strings"
)

const MOVEFILE_DELAY_UNTIL_REBOOT = 4

// normPath transforms a windows path into an extended-length path as described in
// https://msdn.microsoft.com/en-us/library/windows/desktop/aa365247(v=vs.85).aspx#maxpath
func normPath(path string) (string, error) {
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

// Create calls os.Create with name normalized
func Create(name string) (*os.File, error) {
	name, err := normPath(name)
	if err != nil {
		return nil, err
	}
	return os.Create(name)
}
