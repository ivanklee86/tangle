//go:build e2e

package argocd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setup loads the live ArgoCD auth tokens `task argocd:token` mints into
// .env. Every test in this file needs a real ArgoCD reachable at
// localhost:8080 (see `task services:cicd`) — ArgoCDClient is the thing that
// talks real gRPC-web to ArgoCD, so there's nothing left to fake underneath
// it without just testing our own assumptions about ArgoCD's behavior.
func setup(t *testing.T) {
	err := godotenv.Load("../../.env")
	if err != nil {
		t.Fatal(err)
	}
}

// TestE2E_NewArgoCDClient_ValidOptions is split out from the unit-tested
// TestNewArgoCDClient (client_test.go, no build tag — always compiled, so a
// same-named function here would collide when building with -tags=e2e).
// Unlike the "invalid options" case, which only exercises this package's own
// missing-token-env-var validation, this one wants a real ArgoCD: it asserts
// the client is actually usable, which the constructor alone no longer tells
// you. NewArgoCDClient dials eagerly but treats failure as a warning and
// connects lazily instead (ADR 0025), and with gRPC-Web that dial only reaches
// the local proxy socket anyway — it never contacts ArgoCD. So the List below,
// not the constructor, is what proves the connection works.
//
// Every top-level test in this file is prefixed TestE2E_ (matching
// internal/tangle/server_e2e_test.go's TestE2E_Server*) so `-run '^TestE2E_'`
// (tasks/go.yaml's test:e2e/test:e2e:ci) can select just these, instead of
// also re-running every untagged unit/integration test that -tags=e2e still
// compiles alongside them.
func TestE2E_NewArgoCDClient_ValidOptions(t *testing.T) {
	setup(t)

	got, err := NewArgoCDClient(&ArgoCDClientOptions{
		Address:         "localhost:8080",
		PlainText:       true,
		AuthTokenEnvVar: "ARGOCD_TOKEN",
	})
	assert.NoError(t, err)
	assert.NotNil(t, got)
	defer func() { _ = got.Close() }()

	_, err = got.List(context.Background(), &application.ApplicationQuery{})
	assert.NoError(t, err, "a client built from valid options should be able to talk to ArgoCD")
}

func TestE2E_ArgoCDClient_List(t *testing.T) {
	setup(t)

	labelQueryInclude := "foo=bar"
	labelQueryExclude := "foo=bar,env!=test"

	tests := []struct {
		name          string
		options       *ArgoCDClientOptions
		query         *application.ApplicationQuery
		resultsLength int
		wantErr       bool
	}{
		{
			name: "lists applications successfully",
			options: &ArgoCDClientOptions{
				Address:         "localhost:8080",
				PlainText:       true,
				AuthTokenEnvVar: "ARGOCD_TOKEN",
			},
			query: &application.ApplicationQuery{
				Selector: &labelQueryInclude,
			},
			resultsLength: 2,
			wantErr:       false,
		},
		{
			name: "lists applications with exclude labels",
			options: &ArgoCDClientOptions{
				Address:         "localhost:8080",
				PlainText:       true,
				AuthTokenEnvVar: "ARGOCD_TOKEN",
			},
			query: &application.ApplicationQuery{
				Selector: &labelQueryExclude,
			},
			resultsLength: 1,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewArgoCDClient(tt.options)
			assert.NoError(t, err)

			got, err := client.List(context.Background(), tt.query)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
			assert.Len(t, got.Items, tt.resultsLength)
		})
	}
}

func TestE2E_ArgoCDClient_GetApplicationManifests(t *testing.T) {
	setup(t)

	applicationName := "test-1"

	tests := []struct {
		name    string
		options *ArgoCDClientOptions
		query   *application.ApplicationManifestQuery
		wantErr bool
	}{
		{
			name: "gets application manifests successfully",
			options: &ArgoCDClientOptions{
				Address:         "localhost:8080",
				PlainText:       true,
				AuthTokenEnvVar: "ARGOCD_TOKEN",
			},
			query: &application.ApplicationManifestQuery{
				Name: &applicationName,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewArgoCDClient(tt.options)
			assert.NoError(t, err)

			got, err := client.GetApplicationManifests(context.Background(), tt.query)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

func TestE2E_ArgoCDClient_Get(t *testing.T) {
	setup(t)
	applicationName := "test-1"
	refresh := "hard"

	tests := []struct {
		name    string
		options *ArgoCDClientOptions
		query   *application.ApplicationQuery
		wantErr bool
	}{
		{
			name: "gets application successfully",
			options: &ArgoCDClientOptions{
				Address:         "localhost:8080",
				PlainText:       true,
				AuthTokenEnvVar: "ARGOCD_TOKEN",
			},
			query: &application.ApplicationQuery{
				Name:    &applicationName,
				Refresh: &refresh,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewArgoCDClient(tt.options)
			assert.NoError(t, err)

			got, err := client.Get(context.Background(), tt.query)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, got)
		})
	}
}

// isolateTempDir points os.TempDir() at a directory private to this test.
//
// argo-cd's apiclient builds each gRPC-Web proxy's socket path from
// os.TempDir() (pkg/apiclient/grpcproxy.go), which re-reads $TMPDIR on every
// call, so this is what makes the socket assertions below deterministic. They
// can't just diff /tmp before and after: `go test ./...` runs package binaries
// in parallel, and internal/tangle's e2e tests build 14 ArgoCD clients of their
// own, each dropping an argocd-*.sock into the same directory at an
// unpredictable moment.
//
// The directory name is kept short on purpose — a unix socket path is limited
// to ~108 characters and t.TempDir() bakes the (long) test name into it.
func isolateTempDir(t *testing.T) {
	t.Helper()

	dir, err := os.MkdirTemp("", "tangle-e2e")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	t.Setenv("TMPDIR", dir)
}

// proxySockets lists the gRPC-Web proxy sockets this test's clients have open.
// One connection means one socket; closing the connection stops the proxy and
// unlinks the file.
func proxySockets(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(os.TempDir(), "argocd-*.sock"))
	if err != nil {
		t.Fatal(err)
	}

	return paths
}

// TestE2E_ArgoCDClient_ReconnectsAfterConnectionLoss is the regression test for
// the bug in ADR 0025: a client whose connection has gone away used to fail
// every subsequent request for the life of the process.
//
// It forces the reconnect directly rather than waiting for a real transport
// death — the production trigger is gRPC's 30-minute idle timeout, and the
// error that surfaces from it (a client preface written to a closed socket)
// can't be provoked from outside the SDK. Classification of that error is
// covered by the unit tests; what this proves is the half those can't: that a
// replacement connection dialed against a live ArgoCD actually serves requests,
// and that the old proxy is gone afterwards.
func TestE2E_ArgoCDClient_ReconnectsAfterConnectionLoss(t *testing.T) {
	setup(t)
	isolateTempDir(t)

	client, err := NewArgoCDClient(&ArgoCDClientOptions{
		Name:            "reconnect-test",
		Address:         "localhost:8080",
		PlainText:       true,
		AuthTokenEnvVar: "ARGOCD_TOKEN",
	})
	assert.NoError(t, err)
	defer func() { _ = client.Close() }()

	_, err = client.List(context.Background(), &application.ApplicationQuery{})
	assert.NoError(t, err)

	argoCDClient, ok := client.(*ArgoCDClient)
	assert.True(t, ok)

	stale, err := argoCDClient.connection()
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), stale.generation)

	// require, not assert: the assertion below indexes into this slice, so an
	// unexpected length has to stop the test rather than panic it.
	original := proxySockets(t)
	require.Len(t, original, 1, "the client should own exactly one gRPC-Web proxy socket")

	fresh, err := argoCDClient.reconnect(stale)
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), fresh.generation)

	got, err := client.List(context.Background(), &application.ApplicationQuery{})
	assert.NoError(t, err, "the client should serve requests again after reconnecting")
	assert.NotNil(t, got)

	assert.NoFileExists(t, original[0], "reconnecting should release the previous proxy, not leak it")

	// A stale caller racing the reconnect gets the current connection back
	// instead of dialing a third one.
	same, err := argoCDClient.reconnect(stale)
	assert.NoError(t, err)
	assert.Equal(t, uint64(2), same.generation)
}

// TestE2E_ArgoCDClient_CloseReleasesTheProxy covers the leak half of ADR 0025:
// the io.Closer returned by NewApplicationClient used to be discarded, so the
// proxy's grpc.Server, its goroutine and its socket outlived every client.
func TestE2E_ArgoCDClient_CloseReleasesTheProxy(t *testing.T) {
	setup(t)
	isolateTempDir(t)

	client, err := NewArgoCDClient(&ArgoCDClientOptions{
		Name:            "close-test",
		Address:         "localhost:8080",
		PlainText:       true,
		AuthTokenEnvVar: "ARGOCD_TOKEN",
	})
	assert.NoError(t, err)

	added := proxySockets(t)
	require.Len(t, added, 1, "the client should own exactly one gRPC-Web proxy socket")

	assert.NoError(t, client.Close())
	assert.NoFileExists(t, added[0], "Close should stop the gRPC-Web proxy and unlink its socket")

	_, err = client.List(context.Background(), &application.ApplicationQuery{})
	assert.ErrorIs(t, err, ErrClientClosed)
}
