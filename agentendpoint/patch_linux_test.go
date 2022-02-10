//  Copyright 2022 Google Inc. All Rights Reserved.
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
	"reflect"
	"regexp"
	"testing"

	"github.com/GoogleCloudPlatform/osconfig/ospatch"
)

func TestExcludeConversion(t *testing.T) {
	strictString := "PackageName"
	excludeStrictString := ospatch.Exclude{IsRegexp: false, StrictString: &strictString}
	regex, _ := regexp.Compile("PackageName")
	excludeRegex := ospatch.Exclude{IsRegexp: true, Regex: regex}
	emptyRegex, _ := regexp.Compile("")
	emptyExcludeRegex := ospatch.Exclude{IsRegexp: true, Regex: emptyRegex}
	slashString := "/"
	slashExcludeStrictString := ospatch.Exclude{IsRegexp: false, StrictString: &slashString}
	emptyString := ""
	emptyExcludeStrictString := ospatch.Exclude{IsRegexp: false, StrictString: &emptyString}

	tests := []struct {
		name  string
		input []string
		want  []*ospatch.Exclude
	}{
		{name: "StrictStringConversion", input: []string{"PackageName"}, want: []*ospatch.Exclude{&excludeStrictString}},
		{name: "RegexConversion", input: []string{"/PackageName/"}, want: []*ospatch.Exclude{&excludeRegex}},
		{name: "CornerCaseRegex", input: []string{"//"}, want: []*ospatch.Exclude{&emptyExcludeRegex}},
		{name: "CornerCaseStrictString", input: []string{"/"}, want: []*ospatch.Exclude{&slashExcludeStrictString}},
		{name: "CornerCaseEmptyString", input: []string{""}, want: []*ospatch.Exclude{&emptyExcludeStrictString}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			excludes, err := convertInputToExcludes(tt.input)
			if err != nil {
				t.Errorf("err = %v, want %v", err, nil)
			}
			if !reflect.DeepEqual(excludes, tt.want) {
				t.Errorf("convertInputToExcludes() = %v, want = %v", excludes, tt.want)
			}
		})
	}
}
