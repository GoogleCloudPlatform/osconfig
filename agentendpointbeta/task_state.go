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

package agentendpointbeta

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

type taskState struct {
	PatchTask *patchTask `json:",omitempty"`
	ExecTask  *execTask  `json:",omitempty"`
}

func saveState(st *taskState, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if st == nil {
		return writeFile(path, []byte("{}"))
	}

	d, err := json.Marshal(st)
	if err != nil {
		return err
	}

	return writeFile(path, d)
}

func loadState(path string) (*taskState, error) {
	d, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var st taskState
	return &st, json.Unmarshal(d, &st)
}

func writeFile(path string, data []byte) error {
	// Write state to a temporary file first.
	tmp, err := ioutil.TempFile(filepath.Dir(path), "")
	if err != nil {
		return err
	}
	newStateFile := tmp.Name()

	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	// Move the new temp file to the live path.
	return os.Rename(newStateFile, path)
}
