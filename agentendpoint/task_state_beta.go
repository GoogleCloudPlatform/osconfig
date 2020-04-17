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

package agentendpoint

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
)

type taskStateBeta struct {
	PatchTask *patchTaskBeta `json:",omitempty"`
	ExecTask  *execTaskBeta  `json:",omitempty"`
}

func (s *taskStateBeta) save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	if s == nil {
		return writeFile(path, []byte("{}"))
	}

	d, err := json.Marshal(s)
	if err != nil {
		return err
	}

	return writeFile(path, d)
}

func loadStateBeta(path string) (*taskStateBeta, error) {
	d, err := ioutil.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var st taskStateBeta
	return &st, json.Unmarshal(d, &st)
}
