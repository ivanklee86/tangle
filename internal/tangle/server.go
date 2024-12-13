package tangle

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hellofresh/health-go/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/ivanklee86/tangle/internal/argocd"
)

type Tangle struct {
	Server        *http.Server
	Config        *TangleConfig
	ArgoCDClients map[string]argocd.IArgoCDClient
	Log           *zap.SugaredLogger
}

func New(config *TangleConfig) *Tangle {
	tangle := Tangle{}
	tangle.Config = config

	// set up logging
	logger, _ := zap.NewProduction()
	defer logger.Sync() //nolint:all
	tangle.Log = logger.Sugar()

	// Create ArgoCD clients
	clients := make(map[string]argocd.IArgoCDClient)
	for key, value := range config.ArgoCDs {
		client, _ := argocd.New(&argocd.ArgoCDClientOptions{
			Address:                value.Address,
			Insecure:               value.Insecure,
			AuthTokenEnvVar:        value.AuthTokenEnvVar,
			DoNotInstrumentWorkers: tangle.Config.DoNotInstrumentWorkers,
		})

		clients[key] = client
	}
	tangle.ArgoCDClients = clients

	mux := http.NewServeMux()
	// Set up Server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", tangle.Config.Port),
		Handler: mux,
	}
	tangle.Server = server

	//Set up Prometheus
	mux.Handle("/metrics", promhttp.Handler())

	// Application routes
	mux.HandleFunc("/applications", tangle.applicationsHandler)

	// Set up healthchecks
	h, _ := health.New(health.WithComponent(
		health.Component{
			Name:    "tangle",
			Version: "v1.0",
		},
	))

	mux.Handle("/health", h.Handler())

	return &tangle
}

func (t *Tangle) Start() {
	t.Log.Infoln("Starting server.")
	go func() {
		if err := t.Server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			t.Log.Fatalf("HTTP server error: %v", err)
		}
		t.Log.Infoln("Stopped serving new connections.")
	}()

	// Set up graceful service shutdown.
	gracefulShutdown := make(chan os.Signal, 1)
	signal.Notify(gracefulShutdown, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-gracefulShutdown

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := t.Server.Shutdown(shutdownCtx); err != nil {
		t.Log.Fatalf("HTTP shutdown error: %v", err)
	}
	t.Log.Info("Graceful shutdown complete.")
}
