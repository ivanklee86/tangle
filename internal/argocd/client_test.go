package argocd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewArgoCDClient/creates client with invalid options is the one
// TestNewArgoCDClient case that doesn't need a live ArgoCD: ARGOCD_TOKEN1 is
// a deliberately-nonexistent env var name, so NewArgoCDClient's
// os.LookupEnv check fails before any network call is attempted — no
// .env/t.Setenv needed, since nothing in this environment ever sets that
// name. The "valid options" case moved to client_e2e_test.go: unlike this
// one, it needs a real ArgoCD to dial successfully (see that file's comment).
func TestNewArgoCDClient(t *testing.T) {
	tests := []struct {
		name    string
		options *ArgoCDClientOptions
		wantErr bool
	}{
		{
			name: "creates client with invalid options",
			options: &ArgoCDClientOptions{
				Address:         "https://localhost:8080",
				PlainText:       true,
				AuthTokenEnvVar: "ARGOCD_TOKEN1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewArgoCDClient(tt.options)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}
