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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"
)

const (
	recipeBasePath = "/tmp/osconfig_software_recipes"
	// TODO: move to constants, split linux and windows.
)

// InstallRecipe installs a recipe.
func InstallRecipe(ctx context.Context, recipe osconfigpb.SoftwareRecipe) error {
	steps := recipe.InstallSteps
	recipeDB := newRecipeDB()
	installedRecipe, ok := recipeDB.GetRecipe(recipe.Name)
	if ok {
		if (!installedRecipe.Greater(recipe.Version)) &&
			(recipe.DesiredState == osconfigpb.DesiredState_UPDATED) {
			steps = recipe.UpdateSteps
		} else {
			return nil
		}
	}
	runDir, err := createBaseDir(recipe, "runID")
	if err != nil {
		return err
	}
	artifacts, err := FetchArtifacts(ctx, recipe.Artifacts, runDir)
	if err != nil {
		return err
	}
	for idx, step := range steps {
		cmd, err := BuildCommand(step, artifacts)
		if err != nil {
			return fmt.Errorf("Got error in step #%d: %s", ids, err)
		}
		cmdObj := exec.Command(cmd[0], cmd[1:]...)

		cmdObj.Dir = filepath.Join(runDir, "stepName")
		if err := os.MkdirAll(cmdObj.Dir, os.ModeDir|0755); err != nil {
			return fmt.Errorf("failed to create working dir for step %d: %s", idx, err)
		}
		envs := []string{
			fmt.Sprintf("RECIPE_NAME=%s", recipe.Name),
			fmt.Sprintf("RECIPE_VERSION=%s", recipe.Version),
			fmt.Sprintf("RUNID=%s", "runId"),
			fmt.Sprintf("PWD=%s", cmdObj.Dir),
		}
		cmdObj.Env = append(cmdObj.Env, envs...)
		for artifactID, artifactPath := range artifacts {
			cmdObj.Env = append(cmdObj.Env, fmt.Sprintf("%s=%s", artifactID, artifactPath))
		}
		// TODO: log output from command.
		_, err = cmdObj.Output()
		if err != nil {
			return err
		}
	}
	return recipeDB.AddRecipe(recipe.Name, recipe.Version)
}

func createBaseDir(recipe osconfigpb.SoftwareRecipe, runID string) (string, error) {
	dirName := recipe.Name
	if recipe.Version != "" {
		dirName = fmt.Sprintf("%s_%s", dirName, recipe.Version)
	}
	fullPath := filepath.Join(recipeBasePath, dirName, runID)

	if err := os.MkdirAll(fullPath, os.ModeDir|0755); err != nil {
		return "", fmt.Errorf("failed to create working dir for recipe: %q %s", recipe.Name, err)
	}

	return fullPath, nil
}
