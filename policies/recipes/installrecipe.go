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
	"path/filepath"
	"time"

	"github.com/GoogleCloudPlatform/guest-logging-go/logger"

	agentendpointpb "google.golang.org/genproto/googleapis/cloud/osconfig/agentendpoint/v1beta"
)

var (
	recipeBasePath = filepath.Join(os.TempDir(), "osconfig_software_recipes")
)

// InstallRecipe installs a recipe.
func InstallRecipe(ctx context.Context, recipe *agentendpointpb.SoftwareRecipe) error {
	steps := recipe.InstallSteps
	recipeDB, err := newRecipeDB()
	if err != nil {
		return err
	}
	installedRecipe, ok := recipeDB.getRecipe(recipe.Name)
	if ok {
		logger.Debugf("Currently installed version of software recipe %s with version %s.", recipe.Name, installedRecipe.Version)
		if (installedRecipe.compare(recipe.Version)) && (recipe.DesiredState == agentendpointpb.DesiredState_UPDATED) {
			logger.Infof("Upgrading software recipe %s from version %s to %s.", recipe.Name, installedRecipe.Version, recipe.Version)
			steps = recipe.UpdateSteps
		} else {
			logger.Debugf("Skipping software recipe %s.", recipe.Name)
			return nil
		}
	} else {
		logger.Infof("Installing software recipe %s.", recipe.Name)
	}

	logger.Debugf("Creating working directory for recipe %s.", recipe.Name)
	runID := fmt.Sprintf("run_%d", time.Now().UnixNano())
	runDir, err := createBaseDir(recipe, runID)
	if err != nil {
		return fmt.Errorf("failed to create base directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(runDir); err != nil {
			logger.Warningf("Failed to remove recipe working directory at %q: %v", runDir, err)
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
		logger.Debugf("Running step %d: %q", i, step)
		stepDir := filepath.Join(runDir, fmt.Sprintf("step%02d", i))
		if err := os.MkdirAll(stepDir, 0755); err != nil {
			return fmt.Errorf("failed to create recipe step dir %q: %s", stepDir, err)
		}

		var err error
		switch {
		case step.GetFileCopy() != nil:
			err = stepCopyFile(step.GetFileCopy(), artifacts, runEnvs, stepDir)
		case step.GetArchiveExtraction() != nil:
			err = stepExtractArchive(step.GetArchiveExtraction(), artifacts, runEnvs, stepDir)
		case step.GetMsiInstallation() != nil:
			err = stepInstallMsi(step.GetMsiInstallation(), artifacts, runEnvs, stepDir)
		case step.GetFileExec() != nil:
			err = stepExecFile(step.GetFileExec(), artifacts, runEnvs, stepDir)
		case step.GetScriptRun() != nil:
			err = stepRunScript(step.GetScriptRun(), artifacts, runEnvs, stepDir)
		case step.GetDpkgInstallation() != nil:
			err = stepInstallDpkg(step.GetDpkgInstallation(), artifacts)
		case step.GetRpmInstallation() != nil:
			err = stepInstallRpm(step.GetRpmInstallation(), artifacts)
		default:
			err = fmt.Errorf("unknown step type for step %d", i)
		}
		if err != nil {
			recipeDB.addRecipe(recipe.Name, recipe.Version, false)
			return err
		}
	}

	logger.Infof("All steps completed successfully, marking recipe %s as installed.", recipe.Name)
	return recipeDB.addRecipe(recipe.Name, recipe.Version, true)
}

func createBaseDir(recipe *agentendpointpb.SoftwareRecipe, runID string) (string, error) {
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
