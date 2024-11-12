package main

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {
	t.Run("Basic test.", func(t *testing.T) {
		app := New()

		req := httptest.NewRequest("GET", "http://localhost:8081", nil)
		resp, _ := app.Test(req)

		if resp.StatusCode == fiber.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			assert.Equal(t, "Hello world!", string(body))
		}
	})
}
