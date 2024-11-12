package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/monitor"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
)

func New() *fiber.App {
	app := fiber.New()

	// Initialize health and monitoring
	app.Use(healthcheck.New())
	app.Get("/metrics", monitor.New())

	// Application routes
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello world!")
	})

	return app
}

func main() {
	app := New()
	log.Fatal(app.Listen(":8081"))
}
