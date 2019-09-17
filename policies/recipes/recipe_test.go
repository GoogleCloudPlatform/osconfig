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
	"strings"
	"testing"
)

func TestRecipe_SetVersion_ValidVersion(t *testing.T) {
	rec := &Recipe{}
	rec.setVersion("1.2.3.23")
	if rec.Version[0] != 1 || rec.Version[1] != 2 || rec.Version[2] != 3 || rec.Version[3] != 23 {
		t.Errorf("invalid Version set for the recipe")
	}
}

func TestRecipe_SetVersion_EmptyVersion(t *testing.T) {
	rec := &Recipe{}
	rec.setVersion("")
	if len(rec.Version) != 1 || rec.Version[0] != 0 {
		t.Errorf("invalid Version set for the recipe")
	}
}

func TestRecipe_SetVersion_InvalidVersion(t *testing.T) {
	rec := &Recipe{}
	err := rec.setVersion("12.32.23.23.23.23")
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("setVersion should return error")
	}

}

func TestRecipe_SetVersion_InvalidCharacter(t *testing.T) {
	rec := &Recipe{}
	err := rec.setVersion("12.32.dsf.23")
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("setVersion should return error")
	}
}

func TestRecipe_Compare_EmptyVersion(t *testing.T) {
	rec := &Recipe{Version: []int{1, 2, 3, 4}}
	if rec.compare("") {
		t.Errorf("should return false")
	}
}

func TestRecipe_Compare_InvalidInput(t *testing.T) {
	rec := &Recipe{Version: []int{1, 2, 3, 4}}
	if rec.compare("1.2.3.4.56.7") {
		t.Errorf("should return false")
	}
}

func TestRecipe_Compare_PaddingNeededAndCompare(t *testing.T) {
	rec := &Recipe{Version: []int{1, 2, 3, 4}}
	if !rec.compare("1.3") {
		t.Errorf("should return true")
	}
}

func TestRecipe_Compare_PaddingNeededToRecipe(t *testing.T) {
	rec := &Recipe{Version: []int{1, 3}}
	if rec.compare("1.2.3.4") {
		t.Errorf("should return false")
	}
}

func TestRecipe_Compare_IsGreater(t *testing.T) {
	rec := &Recipe{Version: []int{1, 2, 3, 4}}
	if !rec.compare("1.3.4.2") {
		t.Errorf("should return true")
	}
}

func TestRecipe_Compare_IsNotGreater(t *testing.T) {
	rec := &Recipe{Version: []int{1, 6, 3, 4}}
	if rec.compare("1.3.4.2") {
		t.Errorf("should return false")
	}
}

func TestRecipe_Compare_IsEqualVersion(t *testing.T) {
	rec := &Recipe{Version: []int{1, 6, 3, 4}}
	if rec.compare("1.6.3.4") {
		t.Errorf("should return false")
	}
}
