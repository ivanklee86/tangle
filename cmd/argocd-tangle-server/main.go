package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/monitor"
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

	// Set up graceful service shutdown.
	gracefulShutdown := make(chan os.Signal, 1)
	signal.Notify(gracefulShutdown, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	
	go func() {
		signal := <- gracefulShutdown
		log.Fatalf("Shutting down.  Signal: %s", signal)
		_ = app.Shutdown()
	}()

	// Start service.
	if err := app.Listen(":8081"); err != nil {
		log.Error(err)
	}

	// Cleanup tasks.
}
