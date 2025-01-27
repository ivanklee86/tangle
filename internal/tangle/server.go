package tangle

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flowchartsman/swaggerui"
	"github.com/hellofresh/health-go/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yarlson/chiprom"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httplog/v2"

	"github.com/ivanklee86/tangle/internal/argocd"
)

type Tangle struct {
	Server  *http.Server
	Config  *TangleConfig
	ArgoCDs map[string]argocd.IArgoCDWrapper
	Log     *httplog.Logger
}

var (
	// Injected at build time.
	version = "package_default"
	//go:embed swagger.json
	spec []byte
)

func New(config *TangleConfig, version string) *Tangle {
	tangle := Tangle{}
	tangle.Config = config

	// set up logging
	logger := httplog.NewLogger("tangle", httplog.Options{
		// JSON:             true,
		LogLevel:         slog.LevelDebug,
		Concise:          true,
		RequestHeaders:   false,
		MessageFieldName: "message",
		// TimeFieldFormat: time.RFC850,
		Tags: map[string]string{
			"version": version,
			"env":     config.Env,
		},
		QuietDownRoutes: []string{
			"/",
			"/metrics",
			"/swagger",
			"/health",
		},
		QuietDownPeriod: 10 * time.Second,
	})
	tangle.Log = logger

	// Create ArgoCD clients
	wrappers := make(map[string]argocd.IArgoCDWrapper)
	for key, value := range config.ArgoCDs {
		client, _ := argocd.NewArgoCDClient(&argocd.ArgoCDClientOptions{
			Address:         value.Address,
			Insecure:        value.Insecure,
			AuthTokenEnvVar: value.AuthTokenEnvVar,
		})

		wrapper, _ := argocd.New(client, key, &argocd.ArgoCDWrapperOptions{
			DoNotInstrumentWorkers: tangle.Config.DoNotInstrument,
		})

		wrappers[key] = wrapper
	}
	tangle.ArgoCDs = wrappers

	router := chi.NewRouter()
	// Server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", tangle.Config.Port),
		Handler: router,
	}
	tangle.Server = server

	// Middlewares
	router.Use(httplog.RequestLogger(logger))
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(time.Duration(config.Timeout) * time.Second))
	if !config.DoNotInstrument {
		router.Use(chiprom.NewPatternMiddleware("tangle"))
	}
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	// Metrics
	router.Handle("/metrics", promhttp.Handler())

	// Application routes
	router.Route("/api", func(r chi.Router) {
		r.Get("/applications", tangle.applicationsHandler)
		r.Post("/argocd/{argocd}/applications/{name}/diffs", tangle.applicationManifestsHandler)
	})

	router.Mount("/swagger", http.StripPrefix("/swagger", swaggerui.Handler(spec)))

	// Healthcheck
	h, _ := health.New(health.WithComponent(
		health.Component{
			Name:    "tangle",
			Version: version,
		},
	))
	router.Handle("/health", h.Handler())

	fileDir := http.Dir("./build")
	router.Handle("/*", http.FileServer(fileDir))

	return &tangle
}

func (t *Tangle) Start() {
	t.Log.Info("Starting server.")
	go func() {
		if err := t.Server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			t.Log.Error("HTTP server error.", httplog.ErrAttr(err))
		}
		t.Log.Info("Stopped serving new connections.")
	}()

	// Set up graceful service shutdown.
	gracefulShutdown := make(chan os.Signal, 1)
	signal.Notify(gracefulShutdown, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-gracefulShutdown

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := t.Server.Shutdown(shutdownCtx); err != nil {
		t.Log.Error("HTTP shutdown error", httplog.ErrAttr(err))
	}
	t.Log.Info("Graceful shutdown complete.")
}
