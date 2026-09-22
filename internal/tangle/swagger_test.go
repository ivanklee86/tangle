package tangle

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSwaggerSpecDocumentsApplicationsResponses guards the generated OpenAPI
// spec, which is gitignored and rebuilt by `task go:generate` — so a dropped
// annotation in internal/docs/docs.go never shows up in a diff and would go
// unnoticed until someone read the served /swagger page.
//
// The promise to callers is that every status /api/applications can return is
// documented; 400 is the one #241 added.
func TestSwaggerSpecDocumentsApplicationsResponses(t *testing.T) {
	var parsed struct {
		Paths map[string]struct {
			Get struct {
				Responses  map[string]any `json:"responses"`
				Parameters []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				} `json:"parameters"`
			} `json:"get"`
		} `json:"paths"`
	}

	assert.NoError(t, json.Unmarshal(spec, &parsed), "embedded swagger.json should parse")

	applications, ok := parsed.Paths["/api/applications"]
	assert.True(t, ok, "spec should document /api/applications")

	for _, status := range []string{"200", "400", "500"} {
		assert.Contains(t, applications.Get.Responses, status)
	}

	// The format is the part callers have to get right, and the only place
	// it's written down for an API consumer.
	descriptions := map[string]string{}
	for _, param := range applications.Get.Parameters {
		descriptions[param.Name] = param.Description
	}

	for _, name := range []string{"labels", "excludeLabels"} {
		assert.Contains(t, descriptions, name)
		assert.Contains(t, descriptions[name], "key:value", "%s should document its format", name)
	}
}
