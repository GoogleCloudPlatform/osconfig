package e2etestcompute

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	daisyCompute "github.com/GoogleCloudPlatform/compute-image-tools/daisy/compute"
	"google.golang.org/api/compute/v1"
)

const (
	metadataURL = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/?recursive=true&alt=json"
)

// GetMetadata gets instance attributes from metadata.
func GetMetadata() (map[string]string, error) {
	var md map[string]string
	if md != nil {
		return md, nil
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", metadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("error building HTTP request for metadata: %s", err)
	}
	req.Header.Add("Metadata-Flavor", "Google")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error getting metadata: %s", err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("error getting metadata: %s", err)
	}
	err = json.Unmarshal(body, &md)
	if err != nil {
		return nil, fmt.Errorf("error parsing metadata: %s", err)
	}
	return md, nil
}

func GetCommonInstanceMetadata(client daisyCompute.Client, project string) (*compute.Metadata, error) {
	proj, err := client.GetProject(project)
	if err != nil {
		return nil, fmt.Errorf("error getting project: %v", err)
	}

	return proj.CommonInstanceMetadata, nil
}
