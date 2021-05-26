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

package attributes

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/packages"
)

func TestPostAttributeHappyCase(t *testing.T) {
	testData := "test bytes"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(bytes.Buffer)
		buf.ReadFrom(r.Body)
		newStr := buf.String()

		if strings.Compare(testData, newStr) != 0 {
			// this is just a way to notify client that the data
			// recieved was different than what was sent
			w.WriteHeader(http.StatusExpectationFailed)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()
	if err := PostAttribute(ts.URL, strings.NewReader(testData)); err != nil {
		// PostAttribute throw error if status is not 200
		t.Errorf("test failed, should not be an error; got(%s)", err.Error())
	}
}

func TestPostAttributeInvalidUrl(t *testing.T) {
	err := PostAttribute("http://foo.com/ctl\x80", nil)
	if err == nil {
		t.Errorf("test failed, Should be an error")
	}
}

func TestPostAttributeStatusNotOk(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()
	err := PostAttribute(ts.URL, nil)
	if err == nil || !strings.Contains(err.Error(), "400 Bad Request") {
		t.Errorf("test failed, Should be (400 bad request; got(%+v))", err)
	}
}

func TestPostAttributeCompressedhappyCase(t *testing.T) {
	td := packages.Packages{
		Apt: []*packages.PkgInfo{
			{
				Version: "1.2.3",
				Name:    "test-package",
				Arch:    "amd64",
			},
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusExpectationFailed)
			w.Write([]byte("error reading body"))
			return
		}

		pkg, err := getDecompressPackageInfo(string(body))
		if td.Apt[0].Name != pkg.Apt[0].Name {
			w.WriteHeader(http.StatusExpectationFailed)
			w.Write([]byte(fmt.Sprintf("assert failed! expected(%s)! got(%s)!", td.Apt[0].Name, pkg.Apt[0].Name)))
		}
		if td.Apt[0].Version != pkg.Apt[0].Version {
			w.WriteHeader(http.StatusExpectationFailed)
			w.Write([]byte(fmt.Sprintf("assert failed! expected(%s)! got(%s)!", td.Apt[0].Version, pkg.Apt[0].Version)))
		}
		if td.Apt[0].Arch != pkg.Apt[0].Arch {
			w.WriteHeader(http.StatusExpectationFailed)
			w.Write([]byte(fmt.Sprintf("assert failed! expected(%s)! got(%s)!", td.Apt[0].Arch, pkg.Apt[0].Arch)))
		}
	}))

	err := PostAttributeCompressed(ts.URL, td)
	if err != nil {
		t.Errorf("test failed, should not be an error; got(%v)", err)
	}
}

func getDecompressPackageInfo(encoded string) (*packages.Packages, error) {
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("Error decoding base64: %+v", err)
	}

	gzipReader, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		return nil, fmt.Errorf("Error creating gzip reader: %+v", err)
	}
	defer gzipReader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, gzipReader); err != nil {
		return nil, fmt.Errorf("Error reading gzip data: %+v", err)
	}

	var pkgs packages.Packages
	if err := json.Unmarshal(buf.Bytes(), &pkgs); err != nil {
		return nil, fmt.Errorf("Error unmarshalling json data: %+v", err)
	}

	return &pkgs, nil
}
