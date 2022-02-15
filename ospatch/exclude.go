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

package ospatch

import (
	"regexp"
)

// Exclude represents package exclude entry by a user
type Exclude struct {
	isRegexp     bool
	regex        *regexp.Regexp
	strictString *string
}

// CreateRegexExclude returns new Exclude struct that represents exclusion with regex
func CreateRegexExclude(regex *regexp.Regexp) *Exclude {
	return &Exclude{
		isRegexp: true,
		regex:    regex,
	}
}

// CreateStringExclude returns new Exclude struct that represents exclusion with string
func CreateStringExclude(strictString *string) *Exclude {
	return &Exclude{
		isRegexp:     false,
		strictString: strictString,
	}
}

// MatchesName returns if a package with a certain name matches Exclude struct and should be excluded
func (exclude *Exclude) MatchesName(name *string) bool {
	if exclude.isRegexp {
		return exclude.regex.MatchString(*name)
	}
	return *exclude.strictString == *name
}
