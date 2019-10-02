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

package ospatchog

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestGetBtime(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    int
		wantErr bool
	}{
		{"NormalCase", "procs_running 2\nprocs_blocked 0\nctxt 22762852599\nbtime 1561478350\nprocesses 15504510", 1561478350, false},
		{"NoBtime", "procs_running 2\nprocs_blocked 0\nctxt 22762852599\nprocesses 15504510", 0, true},
		{"CantParseInt", "procs_running 2\nprocs_blocked 0\nctxt 22762852599\nbtime notanint\nprocesses 15504510", 0, true},
		{"CantParseLine", "procs_running 2\nprocs_blocked 0\nctxt 22762852599\nbtime1561478350\nprocesses 15504510", 0, true},
	}
	for _, tt := range tests {
		td, err := ioutil.TempDir(os.TempDir(), "")
		if err != nil {
			t.Fatalf("error creating temp dir: %v", err)
		}
		defer os.RemoveAll(td)
		t.Run(tt.name, func(t *testing.T) {
			f, err := ioutil.TempFile(td, "")
			if err != nil {
				t.Fatalf("error creating temp file: %v", err)
			}
			if _, err := f.Write([]byte(tt.in)); err != nil {
				t.Fatalf("error writing temp file: %v", err)
			}
			if err := f.Close(); err != nil {
				t.Fatalf("error writing temp file: %v", err)
			}

			got, err := getBtime(f.Name())
			if (err != nil) != tt.wantErr {
				t.Errorf("getBtime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getBtime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRpmRebootRequired(t *testing.T) {
	type args struct {
		pkgs  []byte
		btime int
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"RebootRequired", args{[]byte("1\n3\n2\n6"), 5}, true},
		{"NoRebootRequired", args{[]byte("1\n3\n2\n5"), 5}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rpmRebootRequired(tt.args.pkgs, tt.args.btime)
			if got != tt.want {
				t.Errorf("rpmRebootRequired() = %v, want %v", got, tt.want)
			}
		})
	}
}
