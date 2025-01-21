package tangle

import (
	"sigs.k8s.io/yaml"

	"github.com/sergi/go-diff/diffmatchpatch"
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

func diffManifests(currentManifest string, compareManifest string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(currentManifest, compareManifest, false)
	patch := dmp.PatchMake(diffs)

	return dmp.PatchToText(patch)
}
