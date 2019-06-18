package recipes

import osconfigpb "github.com/GoogleCloudPlatform/osconfig/_internal/gapi-cloud-osconfig-go/google.golang.org/genproto/googleapis/cloud/osconfig/v1alpha2"

func FetchArtifacts(recipeArtifacts []*osconfigpb.SoftwareRecipe_Artifact) (map[string]string, error) {
	artifacts := make(map[string]string)
	for _, artifact := range recipeArtifacts {
		artifacts[artifact.Id] = artifact.Uri
	}
	return artifacts, nil
}
