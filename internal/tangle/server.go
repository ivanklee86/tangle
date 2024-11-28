package tangle

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"github.com/gofiber/fiber/v2/middleware/healthcheck"
	"github.com/gofiber/fiber/v2/middleware/monitor"
)

type Tangle struct {
	App    *fiber.App
	Config *TangleConfig
}

func New(config *TangleConfig) *Tangle {
	tangle := Tangle{}
	tangle.Config = config

	app := fiber.New()

	//Set up Prometheus
	prometheus := fiberprometheus.New("tangle")
	prometheus.RegisterAt(app, "/metrics")
	prometheus.SetSkipPaths([]string{"/livez", "/readyz", "/console"}) // Optional: Remove some paths from metrics
	app.Use(prometheus.Middleware)

	// Initialize health and monitoring
	app.Use(healthcheck.New())
	app.Get("/console", monitor.New())

	// Application routes
	app.Get("/", func(c *fiber.Ctx) error {
		response := fmt.Sprintf("Hello great %s!", tangle.Config.Name)
		return c.SendString(response)
	})

	tangle.App = app

	return &tangle
}

func (t *Tangle) Start() {
	// Set up graceful service shutdown.
	gracefulShutdown := make(chan os.Signal, 1)
	signal.Notify(gracefulShutdown, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		signal := <-gracefulShutdown
		log.Fatalf("Shutting down.  Signal: %s", signal)
		_ = t.App.Shutdown()
	}()

	// Start service.
	if err := t.App.Listen(fmt.Sprintf(":%d", t.Config.Port)); err != nil {
		log.Error(err)
	}
}
