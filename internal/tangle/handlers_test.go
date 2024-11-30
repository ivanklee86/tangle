package tangle

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHandlers(t *testing.T) {
	config := TangleConfig{
		Name:                   "test-tangle",
		Domain:                 "localhost",
		Port:                   8081,
		DoNotInstrumentWorkers: true,
	}

	t.Run("applications", func(t *testing.T) {
		tangle := New(&config)

		req, _ := http.NewRequest("GET", "/applications?labels=foo:bar", nil)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(tangle.applicationsHandler)
		handler.ServeHTTP(rr, req)

		assert.NotNil(t, rr.Body.String())
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}
