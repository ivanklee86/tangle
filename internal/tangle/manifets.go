package tangle

import (
	"sigs.k8s.io/yaml"
)

func assembleManifests(manifests []string) (*string, error) {
	assembledManifests := ""
	for _, manifest := range manifests {
		k8sObjectYaml, err := yaml.JSONToYAML([]byte(manifest))
		if err != nil {
			return nil, err
		}
		assembledManifests += assembledManifests + "---\n" + string(k8sObjectYaml) + "\n"

	}

	return &assembledManifests, nil
}
