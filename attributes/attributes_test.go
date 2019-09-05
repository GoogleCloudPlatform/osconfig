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

// Package attributes posts data to Guest Attributes.

package attributes

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostAttribute_happyCase(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{}`)
	}))
	defer ts.Close()
	err := PostAttribute(ts.URL, strings.NewReader("test bytes"))
	if err != nil {
		t.Errorf("test failed, should not be an error")
	}
}

func TestPostAttribute_InvalidUrl(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer ts.Close()
	err := PostAttribute("http://foo.com/ctl\x80", strings.NewReader("test bytes"))
	if err == nil {
		t.Errorf("test failed, Should be an error")
	}
}

func TestPostAttribute_StatusNotOk(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()
	err := PostAttribute(ts.URL, strings.NewReader("test bytes"))
	if err == nil || !strings.Contains(err.Error(), "400 Bad Request") {
		t.Errorf("test failed, Should be an error")
	}
}

func TestPostAttributeCompressed_happyCase(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{}`)
	}))
	defer ts.Close()
	err := PostAttributeCompressed(ts.URL, strings.NewReader("test bytes"))
	if err != nil {
		t.Errorf("test failed, should not be an error")
	}
}
