package tangle

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {
	config := TangleConfig{
		Name:            "test-tangle",
		Domain:          "localhost",
		Port:            8081,
		DoNotInstrument: true,
	}

	t.Run("Basic test", func(t *testing.T) {
		tangle := New(&config, "test")

		assert.NotNil(t, tangle.ArgoCDs)
	})
}

// TestResolveClientIP pins what request logs report as "client.ip": the hop
// immediately in front of Tangle (the rightmost X-Forwarded-For entry), never
// a client-supplied entry further left, and the TCP peer when there's nothing
// trustworthy to read.
func TestResolveClientIP(t *testing.T) {
	const peer = "10.0.0.1:54321"

	tests := []struct {
		name string
		xff  []string
		want string
	}{
		{name: "single entry", xff: []string{"203.0.113.7"}, want: "203.0.113.7"},
		{name: "spoofed entries left of the proxy's are ignored", xff: []string{"6.6.6.6, 203.0.113.7"}, want: "203.0.113.7"},
		{name: "last of multiple headers wins", xff: []string{"6.6.6.6", "203.0.113.7"}, want: "203.0.113.7"},
		{name: "v4-mapped IPv6 folds to IPv4", xff: []string{"::ffff:203.0.113.7"}, want: "203.0.113.7"},
		{name: "no header keeps the TCP peer", xff: nil, want: peer},
		{name: "unparseable rightmost entry keeps the TCP peer", xff: []string{"203.0.113.7, not-an-ip"}, want: peer},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			handler := resolveClientIP(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.RemoteAddr
			}))

			req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
			req.RemoteAddr = peer
			for _, v := range tc.xff {
				req.Header.Add("X-Forwarded-For", v)
			}
			handler.ServeHTTP(httptest.NewRecorder(), req)

			assert.Equal(t, tc.want, got)
		})
	}
}
