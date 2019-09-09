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
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecipeDB_AddRecipe(t *testing.T) {
	rdb, err := SetupTestDB()
	if err != nil {
		t.Fatalf("could not setup test db, %+v\n", err)
	}
	err = rdb.AddRecipe("id1", "1.2.3")
	if err != nil {
		t.Fatalf("could not add recipe to db, got(%s)", err.Error())
	}
	rcp, ok := rdb.GetRecipe("id1")
	if !ok {
		t.Fatalf("error fetching from db")
	}
	if strings.Compare(rcp.Name, "id1") != 0 {
		t.Errorf("expected(id1), got(%s)", rcp.Name)
	}
	if strings.Compare(rcp.GetVersionString(), "1.2.3") != 0 {
		t.Errorf("expected(%s), got(%s)", "1.2.3", rcp.GetVersionString())
	}

	if err = tearDown(); err != nil {
		t.Fatalf("error cleaning recipe db directory, got(%s)", err.Error())
	}
}

func TestRecipeDB_InvalidRecipeVersion(t *testing.T) {
	rdb, err := SetupTestDB()
	if err != nil {
		t.Fatalf("could not setup test db, %+v\n", err)
	}
	err = rdb.AddRecipe("id1", "1.2.3.4.5")
	if err == nil || !strings.Contains(err.Error(), "invalid Version string") {
		t.Fatalf("could not add recipe to db, got(%s)", err.Error())
	}

	if err = tearDown(); err != nil {
		t.Fatalf("error cleaning recipe db directory, got(%s)", err.Error())
	}
}

func TestRecipeDB_LoadDBOnAgentRestart(t *testing.T) {
	dbContent := `
{"recipes":{"id1":{"Name":"id1","Version":[1,2,3],"install_time":1568003382}}}
`
	dbDirUnix = "/tmp/google/"
	fname := filepath.Join(dbDirUnix, dbFileName)
	err := os.Mkdir(dbDirUnix, 0755)
	if err != nil {
		t.Fatalf("error creating test db directory, got(%s)", err.Error())
	}
	_, err = os.Create(fname)
	if err != nil {
		t.Fatalf("error creating test db, got(%s)", err.Error())
	}
	err = ioutil.WriteFile(fname, []byte(dbContent), 0666)
	if err != nil {
		t.Fatalf("error writing to test db, got(%s)", err.Error())
	}

	rdb, err := newRecipeDB()
	if err != nil {
		t.Errorf("could not load db, got(%s)", err.Error())
	}
	rcp, ok := rdb.GetRecipe("id1")
	if !ok {
		t.Fatalf("error fetching from db")
	}
	if strings.Compare(rcp.Name, "id1") != 0 {
		t.Errorf("expected(id1), got(%s)", rcp.Name)
	}
	if strings.Compare(rcp.GetVersionString(), "1.2.3") != 0 {
		t.Errorf("expected(%s), got(%s)", "1.2.3", rcp.GetVersionString())
	}

	if err = tearDown(); err != nil {
		t.Fatalf("error cleaning recipe db directory, got(%s)", err.Error())
	}
}

func TestRecipeDB_LoadDBOnAgentRestartCorruptData(t *testing.T) {
	dbContent := `
{"recipes:{d1":{"Name":"id1","Version":[1,2,3],"install_time":1568003382}}}
`
	dbDirUnix = "/tmp/google/"
	fname := filepath.Join(dbDirUnix, dbFileName)
	err := os.Mkdir(dbDirUnix, 0755)
	if err != nil {
		t.Fatalf("error creating test db directory, got(%s)", err.Error())
	}
	_, err = os.Create(fname)
	if err != nil {
		t.Fatalf("error creating test db, got(%s)", err.Error())
	}
	err = ioutil.WriteFile(fname, []byte(dbContent), 0666)
	if err != nil {
		t.Fatalf("error writing to test db, got(%s)", err.Error())
	}

	_, err = newRecipeDB()
	if err == nil || !strings.Contains(err.Error(), "Json Unmarshalling error") {
		t.Errorf("expected json unmarshalling error, got(%s)", err.Error())
	}

	if err = tearDown(); err != nil {
		t.Fatalf("error cleaning recipe db directory, got(%s)", err.Error())
	}
}

func SetupTestDB() (*RecipeDB, error) {
	dbDirUnix = "/tmp/google/"
	return newRecipeDB()
}

func tearDown() error {
	return os.RemoveAll(getDbDir())
}
