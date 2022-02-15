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
	regex, _ := regexp.Compile("PackageName")
	emptyRegex, _ := regexp.Compile("")
	slashString := "/"
	emptyString := ""

	tests := []struct {
		name  string
		input []string
		want  []*ospatch.Exclude
	}{
		{name: "StrictStringConversion", input: []string{"PackageName"}, want: []*ospatch.Exclude{ospatch.CreateStringExclude(&strictString)}},
		{name: "RegexConversion", input: []string{"/PackageName/"}, want: []*ospatch.Exclude{ospatch.CreateRegexExclude(regex)}},
		{name: "CornerCaseRegex", input: []string{"//"}, want: []*ospatch.Exclude{ospatch.CreateRegexExclude(emptyRegex)}},
		{name: "CornerCaseStrictString", input: []string{"/"}, want: []*ospatch.Exclude{ospatch.CreateStringExclude(&slashString)}},
		{name: "CornerCaseEmptyString", input: []string{""}, want: []*ospatch.Exclude{ospatch.CreateStringExclude(&emptyString)}},
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
