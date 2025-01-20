package tangle

import (
	"sigs.k8s.io/yaml"

	"github.com/sergi/go-diff/diffmatchpatch"
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

func diffManifests(currentManifest string, compareManifest string) string {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(currentManifest, compareManifest, false)

	return dmp.DiffToDelta(diffs)
}
