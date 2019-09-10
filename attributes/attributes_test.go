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
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	err := PostAttribute(ts.URL, strings.NewReader("test bytes"))
	if err == nil || !strings.Contains(err.Error(), "400 Bad Request") {
		t.Errorf("test failed, Should be (400 bad request; got(%+v))", err)
	}
}

func TestPostAttributeCompressedhappyCase(t *testing.T) {
	td := "testing-compression"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusExpectationFailed)
			w.Write([]byte("error reading body"))

			return
		}
		buf, err := getCompressData(strings.NewReader(td))
		if strings.Compare(string(body), buf.String()) != 0 {
			w.WriteHeader(http.StatusExpectationFailed)
			w.Write([]byte(fmt.Sprintf("expected(%s); got(%s)", buf.String(), string(body))))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	err := PostAttributeCompressed(ts.URL, strings.NewReader(td))
	if err != nil {
		t.Errorf("test failed, should not be an error; got(%s)", err.Error())
	}
}
