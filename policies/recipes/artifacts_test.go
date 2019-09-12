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
	"net/url"
	"strings"
	"testing"
)

func TestFetchArtifacts_http_InvalidURL(t *testing.T) {
	uri := "ftp://google.com/agent.deb"
	u, err := url.Parse(uri)
	_, err = getHTTPArtifact(nil, *u)
	if err == nil || !strings.Contains(err.Error(), "unsupported protocol scheme") {
		t.Errorf("expected error (unsupported protocol); got(%v)", err)
	}
}

func TestRecipesgetStoragePathWithExtension(t *testing.T) {
	expect := "/tmp/artifact-id-1.txt"
	localpath := getStoragePath("/tmp", "artifact-id-1", ".txt")
	if localpath != expect {
		t.Errorf("Expected(%s); got(%s)", expect, localpath)
	}
}

func TestRecipesgetStoragePathWitouthExtension(t *testing.T) {
	expect := "/tmp/artifact-id-1"
	localpath := getStoragePath("/tmp", "artifact-id-1", "")
	if localpath != expect {
		t.Errorf("Expected(%s); got(%s)", expect, localpath)
	}
}
