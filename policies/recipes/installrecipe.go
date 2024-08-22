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
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/GoogleCloudPlatform/osconfig/clog"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

// InstallRecipe installs a recipe.
func InstallRecipe(ctx context.Context, recipe *agentendpointpb.SoftwareRecipe) error {
	ctx = clog.WithLabels(ctx, map[string]string{"recipe_name": recipe.GetName()})
	steps := recipe.InstallSteps
	recipeDB, err := newRecipeDB()
	if err != nil {
		return err
	}
	installedRecipe, ok := recipeDB.getRecipe(recipe.Name)
	if ok {
		clog.Debugf(ctx, "Currently installed version of software recipe %s with version %s.", recipe.GetName(), installedRecipe.Version)
		if (installedRecipe.compare(recipe.Version)) && (recipe.DesiredState == agentendpointpb.DesiredState_UPDATED) {
			clog.Infof(ctx, "Upgrading software recipe %s from version %s to %s.", recipe.Name, installedRecipe.Version, recipe.GetVersion())
			steps = recipe.UpdateSteps
		} else {
			clog.Debugf(ctx, "Skipping software recipe %s.", recipe.GetName())
			return nil
		}
	} else {
		clog.Infof(ctx, "Installing software recipe %s.", recipe.GetName())
	}

	clog.Debugf(ctx, "Creating working directory for recipe %s.", recipe.GetName())
	runID := fmt.Sprintf("run_%d", time.Now().UnixNano())
	runDir, err := createBaseDir(recipe, runID)
	if err != nil {
		return fmt.Errorf("failed to create base directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(runDir); err != nil {
			clog.Warningf(ctx, "Failed to remove recipe working directory at %q: %v", runDir, err)
		}
	}()
	artifacts, err := fetchArtifacts(ctx, recipe.Artifacts, runDir)
	if err != nil {
		return fmt.Errorf("failed to obtain artifacts: %v", err)
	}

	runEnvs := []string{
		fmt.Sprintf("RECIPE_NAME=%s", recipe.Name),
		fmt.Sprintf("RECIPE_VERSION=%s", recipe.Version),
		fmt.Sprintf("RUNID=%s", runID),
	}
	for artifactID, artifactPath := range artifacts {
		runEnvs = append(runEnvs, fmt.Sprintf("%s=%s", artifactID, artifactPath))
	}

	for i, step := range steps {
		clog.Debugf(ctx, "Running step %d: %q", i, step)
		stepDir := filepath.Join(runDir, fmt.Sprintf("step%02d", i))
		if err := os.MkdirAll(stepDir, 0755); err != nil {
			return fmt.Errorf("failed to create recipe step dir %q: %s", stepDir, err)
		}

		var err error
		var stepType string
		switch {
		case step.GetFileCopy() != nil:
			stepType = "CopyFile"
			err = stepCopyFile(step.GetFileCopy(), artifacts, runEnvs, stepDir)
		case step.GetArchiveExtraction() != nil:
			stepType = "ExtractArchive"
			err = stepExtractArchive(ctx, step.GetArchiveExtraction(), artifacts, runEnvs, stepDir)
		case step.GetMsiInstallation() != nil:
			stepType = "InstallMsi"
			err = stepInstallMsi(ctx, step.GetMsiInstallation(), artifacts, runEnvs, stepDir)
		case step.GetFileExec() != nil:
			stepType = "ExecFile"
			err = stepExecFile(ctx, step.GetFileExec(), artifacts, runEnvs, stepDir)
		case step.GetScriptRun() != nil:
			stepType = "RunScript"
			err = stepRunScript(ctx, step.GetScriptRun(), artifacts, runEnvs, stepDir)
		case step.GetDpkgInstallation() != nil:
			stepType = "InstallDpkg"
			err = stepInstallDpkg(ctx, step.GetDpkgInstallation(), artifacts)
		case step.GetRpmInstallation() != nil:
			stepType = "InstallRpm"
			err = stepInstallRpm(ctx, step.GetRpmInstallation(), artifacts)
		}
		if err != nil {
			recipeDB.addRecipe(recipe.Name, recipe.Version, false)
			if stepType == "" {
				return fmt.Errorf("unknown step type for step %d", i)
			}
			return fmt.Errorf("error running step %d (%s): %v", i, stepType, err)
		}
	}

	clog.Infof(ctx, "All steps completed successfully, marking recipe %s as installed.", recipe.Name)
	return recipeDB.addRecipe(recipe.Name, recipe.Version, true)
}

func createBaseDir(recipe *agentendpointpb.SoftwareRecipe, runID string) (string, error) {
	name := recipe.Name
	if recipe.Version != "" {
		name = fmt.Sprintf("%s_%s", name, recipe.Version)
	}

	dir, err := ioutil.TempDir("", fmt.Sprintf("%s_%s_", name, runID))
	if err != nil {
		return "", fmt.Errorf("failed to create working dir for recipe: %q %s", recipe.Name, err)
	}

	return dir, nil
}
