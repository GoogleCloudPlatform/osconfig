package recipes

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/util"
	"github.com/GoogleCloudPlatform/osconfig/util/utiltest"
)

func Test_newRecipeDB(t *testing.T) {
	tests := []struct {
		name string

		file  string
		setup func(file string) error

		wantRecipes map[string]Recipe
		wantErr     error
	}{
		{
			name: "file does not exists",

			file: "/var/file_does_not_exists",
			setup: func(_ string) error {
				return nil
			},

			wantRecipes: make(map[string]Recipe, 0),
			wantErr:     nil,
		},
		{
			name: "file exists, but empty",
			file: tempFileMust(os.TempDir(), "recipes", os.ModePerm).Name(),
			setup: func(_ string) error {
				return nil
			},

			wantRecipes: nil,
			wantErr:     fmt.Errorf("unexpected end of JSON input"),
		},
		{
			name: "directory set as filepath",
			file: os.TempDir(),
			setup: func(_ string) error {
				return nil
			},

			wantRecipes: nil,
			wantErr:     fmt.Errorf("read %s: is a directory", os.TempDir()),
		},
		{
			name: "file exist with some recipe",
			file: tempFileMust(os.TempDir(), "recipes", os.ModePerm).Name(),
			setup: func(path string) error {
				recipes := []Recipe{
					{
						Name:        "test",
						Version:     []int{1, 1},
						InstallTime: time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC).Unix(),
						Success:     true,
					},
					{
						Name:        "test2",
						Version:     []int{2, 2},
						InstallTime: time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC).Unix(),
						Success:     false,
					},
				}

				raw, err := json.Marshal(recipes)
				if err != nil {
					return err
				}

				fd, err := os.OpenFile(path, os.O_RDWR, os.ModePerm)
				if err != nil {
					return err
				}

				if _, err := fd.Write(raw); err != nil {
					return err
				}
				return nil
			},

			wantRecipes: map[string]Recipe{
				"test": Recipe{
					Name:        "test",
					Version:     []int{1, 1},
					InstallTime: time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC).Unix(),
					Success:     true,
				},
				"test2": Recipe{
					Name:        "test2",
					Version:     []int{2, 2},
					InstallTime: time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC).Unix(),
					Success:     false,
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.setup(tt.file); err != nil {
				t.Errorf("unwanted error in process of setup: %v", err)
			}

			got, gotErr := newRecipeDB(tt.file)
			var gotRecipes map[string]Recipe
			if got != nil {
				gotRecipes = got.recipes
			}

			utiltest.EnsureError(t, tt.wantErr, gotErr)
			utiltest.EnsureResults(t, tt.wantRecipes, gotRecipes)
		})
	}
}

func Test_recipeDB_getRecipe(t *testing.T) {
	recipeDB := &recipeDB{
		file:     tempFileMust(os.TempDir(), "recipes", os.ModePerm).Name(),
		timeFunc: mockTimeFunc,
		recipes: map[string]Recipe{
			"test": Recipe{
				Name:        "test",
				Version:     []int{1, 1},
				InstallTime: time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC).Unix(),
				Success:     true,
			},
			"test2": Recipe{
				Name:        "test2",
				Version:     []int{2, 2},
				InstallTime: time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC).Unix(),
				Success:     false,
			},
		},
	}

	want1 := Recipe{
		Name:        "test",
		Version:     []int{1, 1},
		InstallTime: time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC).Unix(),
		Success:     true,
	}
	got1, ok1 := recipeDB.getRecipe("test")
	utiltest.EnsureResults(t, want1, got1)
	utiltest.EnsureResults(t, true, ok1)

	want2 := Recipe{}
	got2, ok2 := recipeDB.getRecipe("test5")
	utiltest.EnsureResults(t, want2, got2)
	utiltest.EnsureResults(t, false, ok2)
}

func Test_recipeDB_addRecipe(t *testing.T) {
	tests := []struct {
		name string

		db         *recipeDB
		operations []struct {
			recipe  Recipe
			wantErr error
		}
		wantContent string
	}{
		{
			name: "empty db two operations, expect no errors",
			db: &recipeDB{
				file:     tempFileMust(os.TempDir(), "recipes", os.ModePerm).Name(),
				timeFunc: mockTimeFunc,
				recipes:  make(map[string]Recipe, 0),
			},
			operations: []struct {
				recipe  Recipe
				wantErr error
			}{
				{
					recipe: Recipe{
						Name:    "test",
						Version: []int{1, 1},
						Success: true,
					},
					wantErr: nil,
				},
				{
					recipe: Recipe{
						Name:    "test2",
						Version: []int{2, 2},
						Success: false,
					},
					wantErr: nil,
				},
			},
			wantContent: `[{"Name":"test","Version":[1,1],"InstallTime":949408200,"Success":true},{"Name":"test2","Version":[2,2],"InstallTime":949408200,"Success":false}]`,
		},
		{
			name: "db with one entry, second entry added",
			db: &recipeDB{
				file:     tempFileMust(os.TempDir(), "recipes", os.ModePerm).Name(),
				timeFunc: mockTimeFunc,
				recipes: map[string]Recipe{
					"test2": Recipe{
						Name:        "test2",
						Version:     []int{2, 2},
						InstallTime: time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC).Unix(),
						Success:     false,
					},
				},
			},
			operations: []struct {
				recipe  Recipe
				wantErr error
			}{
				{
					recipe: Recipe{
						Name:    "test3",
						Version: []int{3, 3},
						Success: true,
					},
					wantErr: nil,
				},
			},
			wantContent: `[{"Name":"test2","Version":[2,2],"InstallTime":949408200,"Success":false},{"Name":"test3","Version":[3,3],"InstallTime":949408200,"Success":true}]`,
		},
		{
			name: "invalid entry skiped",
			db: &recipeDB{
				file:     tempFileMust(os.TempDir(), "recipes", os.ModePerm).Name(),
				timeFunc: mockTimeFunc,
				recipes:  make(map[string]Recipe, 0),
			},
			operations: []struct {
				recipe  Recipe
				wantErr error
			}{
				{
					recipe: Recipe{
						Name:        "test2",
						Version:     []int{2, 2},
						InstallTime: time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC).Unix(),
						Success:     false,
					},
					wantErr: nil,
				},
				{
					recipe: Recipe{
						Name:    "test3",
						Version: []int{-3},
						Success: true,
					},
					wantErr: fmt.Errorf("invalid Version string"),
				},
			},
			wantContent: `[{"Name":"test2","Version":[2,2],"InstallTime":949408200,"Success":false}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, operation := range tt.operations {
				recipe := operation.recipe
				gotErr := tt.db.addRecipe(recipe.Name, recipe.Version.String(), recipe.Success)
				utiltest.EnsureError(t, operation.wantErr, gotErr)
			}

			gotContent, gotContentErr := readFile(tt.db.file)
			utiltest.EnsureError(t, nil, gotContentErr)
			utiltest.EnsureResults(t, tt.wantContent, string(gotContent))
		})
	}
}

func Test_recipeDB_saveToFS(t *testing.T) {
	tests := []struct {
		name string

		db *recipeDB

		wantContent string
		wantErr     error
	}{
		{
			name: "database with records, properly stored on the fs",
			db: &recipeDB{
				file:     tempFileMust(os.TempDir(), "recipes", os.ModePerm).Name(),
				timeFunc: mockTimeFunc,
				recipes: map[string]Recipe{
					"test": Recipe{
						Name:        "test",
						Version:     []int{1, 1},
						InstallTime: time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC).Unix(),
						Success:     true,
					},
					"test2": Recipe{
						Name:        "test2",
						Version:     []int{2, 2},
						InstallTime: time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC).Unix(),
						Success:     false,
					},
				},
			},
			wantContent: `[{"Name":"test","Version":[1,1],"InstallTime":949408200,"Success":true},{"Name":"test2","Version":[2,2],"InstallTime":949408200,"Success":false}]`,
			wantErr:     nil,
		},
		{
			name: "path to system dir",
			db: &recipeDB{
				file:     string(os.PathSeparator),
				timeFunc: mockTimeFunc,
				recipes:  make(map[string]Recipe, 0),
			},

			wantErr:     fmt.Errorf("createtemp /_*: pattern contains path separator"),
			wantContent: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr := tt.db.saveToFS()
			utiltest.EnsureError(t, tt.wantErr, gotErr)

			if tt.wantErr == nil {
				gotContent, gotContentErr := readFile(tt.db.file)

				utiltest.EnsureError(t, nil, gotContentErr)
				utiltest.EnsureResults(t, string(tt.wantContent), string(gotContent))
			}
		})

	}
}

func Test_newRecipeDBWithDefaults(t *testing.T) {
	wantDir := dbDirUnix
	if runtime.GOOS == "windows" {
		wantDir = dbDirWindows
	}
	wantDBPath := filepath.Join(wantDir, dbFileName)

	db, gotErr := newRecipeDBWithDefaults()
	gotDBPath := db.file

	utiltest.EnsureError(t, nil, gotErr)
	utiltest.EnsureResults(t, wantDBPath, gotDBPath)
}

func tempFileMust(dir, pattern string, mode os.FileMode) *os.File {
	fd, err := util.TempFile(dir, pattern, mode)
	if err != nil {
		panic(err)
	}

	defer fd.Close()

	return fd
}

func readFile(path string) ([]byte, error) {
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return io.ReadAll(fd)
}

func mockTimeFunc() time.Time {
	return time.Date(2000, 2, 1, 12, 30, 0, 0, time.UTC)
}
