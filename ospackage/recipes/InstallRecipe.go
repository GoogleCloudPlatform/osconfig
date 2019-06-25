package recipes

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"

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
	artifacts, err := FetchArtifacts(recipe.Artifacts)
	if err != nil {
		return err
	}
	for idx, step := range steps {
		cmd, err := BuildCommand(step, artifacts)
		if err != nil {
			return err
		}
		cmdObj := exec.Command(cmd[0], cmd[1:]...)
		dirName := recipe.Name
		if recipe.Version != "" {
			dirName = fmt.Sprintf("%s_%s", dirName, recipe.Version)
		}
		cmdObj.Dir = path.Join(recipeBasePath, dirName, "runId", "stepName")
		if err := os.MkdirAll(cmdObj.Dir, os.ModeDir|0755); err != nil {
			return fmt.Errorf("failed to create working dir for step %d: %s", idx, err)
		}
		cmdObj.Env = append(cmdObj.Env, fmt.Sprintf("RECIPE_NAME=%s", recipe.Name))
		cmdObj.Env = append(cmdObj.Env, fmt.Sprintf("RECIPE_VERSION=%s", recipe.Version))
		cmdObj.Env = append(cmdObj.Env, fmt.Sprintf("RUNID=%s", "runId"))
		cmdObj.Env = append(cmdObj.Env, fmt.Sprintf("PWD=%s", cmdObj.Dir))
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
