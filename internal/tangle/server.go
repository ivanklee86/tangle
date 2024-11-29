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
	ArgoCDClients []argocd.ArgoCDClient
	Log           *zap.SugaredLogger
}

func New(config *TangleConfig) *Tangle {
	tangle := Tangle{}
	tangle.Config = config

	// set up logging
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	tangle.Log = logger.Sugar()

	// Create ArgoCD clients
	// TODO: Actually do this!
	tangle.ArgoCDClients = append(tangle.ArgoCDClients, argocd.New(&argocd.ArgoCDClientOptions{}))

	// Set up Server
	server := &http.Server{
		Addr: fmt.Sprintf(":%d", tangle.Config.Port),
	}
	tangle.Server = server

	//Set up Prometheus
	http.Handle("/metrics", promhttp.Handler())

	// Application routes
	http.HandleFunc("/applications", tangle.applicationsHandler)

	// Set up healthchecks
	h, _ := health.New(health.WithComponent(
		health.Component{
			Name:    "tangle",
			Version: "v1.0",
		},
	))

	http.Handle("/health", h.Handler())

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
