package tangle

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/google/uuid"

	"sigs.k8s.io/yaml"
)

func assembleManifests(manifests []string) (*string, error) {
	output := ""
	for _, manifest := range manifests {
		k8sObjectYaml, err := yaml.JSONToYAML([]byte(manifest))
		if err != nil {
			return nil, err
		}
		output += "---\n" + string(k8sObjectYaml)
	}

	return &output, nil
}

func diffManifests(currentManifest string, compareManifest string) (*string, error) {
	tempDir, err := os.MkdirTemp("", "tangle")
	if err != nil {
		return nil, err
	}

	currentFileName := fmt.Sprintf("current_%s.yaml", uuid.New().String())
	compareFileName := fmt.Sprintf("compare_%s.yaml", uuid.New().String())

	currrentFile := path.Join(tempDir, currentFileName)
	currentData, err := yaml.Marshal(currentManifest)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(currrentFile, currentData, 0644)
	if err != nil {
		return nil, err
	}

	compareFile := path.Join(tempDir, compareFileName)
	compareData, err := yaml.Marshal(compareManifest)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(compareFile, compareData, 0644)
	if err != nil {
		return nil, err
	}

	diffBinary := "diff"
	args := []string{currrentFile, compareFile}

	cmd := exec.Command(diffBinary, args...)
	// TODO: Diffs = 1
	output, _ := cmd.Output()
	diffs := string(output)

	return &diffs, nil
}
