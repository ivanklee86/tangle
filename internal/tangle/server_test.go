package tangle

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {
	config := TangleConfig{
		Name:   "test-tangle",
		Domain: "localhost",
		Port:   8081,
	}

	t.Run("Basic test", func(t *testing.T) {
		tangle := New(&config)

		req := httptest.NewRequest("GET", "http://localhost:8081", nil)
		resp, _ := tangle.App.Test(req)

		if resp.StatusCode == fiber.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			assert.Equal(t, "Hello test-tangle!", string(body))
		}
	})
}
