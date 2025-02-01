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

func diffManifests(liveManifest string, targetManifest string) (*string, error) {
	tempDir, err := os.MkdirTemp("", "tangle")
	if err != nil {
		return nil, err
	}

	liveFileName := fmt.Sprintf("live_%s.yaml", uuid.New().String())
	targetFileName := fmt.Sprintf("target_%s.yaml", uuid.New().String())

	liveFile := path.Join(tempDir, liveFileName)
	liveData, err := yaml.Marshal(liveManifest)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(liveFile, liveData, 0644)
	if err != nil {
		return nil, err
	}

	targetFile := path.Join(tempDir, targetFileName)
	targetData, err := yaml.Marshal(targetManifest)
	if err != nil {
		return nil, err
	}
	err = os.WriteFile(targetFile, targetData, 0644)
	if err != nil {
		return nil, err
	}

	diffBinary := "diff"
	args := []string{"-uNar", liveFile, targetFile}

	cmd := exec.Command(diffBinary, args...)
	// TODO: Diffs = 1
	output, _ := cmd.Output()
	diffs := string(output)

	return &diffs, nil
}
