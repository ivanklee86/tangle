package argocd

import (
	"github.com/alitto/pond/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

func instrumentResultPool[P any](name string, argoCDName string, pool pond.ResultPool[P]) {
	poolLabels := make(map[string]string)
	poolLabels["pool"] = name
	poolLabels["argocd"] = argoCDName
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name:        "pool_workers_running",
			Help:        "Number of running worker goroutines",
			ConstLabels: poolLabels,
		},
		func() float64 {
			return float64(pool.RunningWorkers())
		}))
	// Task metrics
	prometheus.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Name:        "pool_tasks_submitted_total",
			Help:        "Number of tasks submitted",
			ConstLabels: poolLabels,
		},
		func() float64 {
			return float64(pool.SubmittedTasks())
		}))
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name:        "pool_tasks_waiting_total",
			Help:        "Number of tasks waiting in the queue",
			ConstLabels: poolLabels,
		},
		func() float64 {
			return float64(pool.WaitingTasks())
		}))
	prometheus.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Name:        "pool_tasks_successful_total",
			Help:        "Number of tasks that completed successfully",
			ConstLabels: poolLabels,
		},
		func() float64 {
			return float64(pool.SuccessfulTasks())
		}))
	prometheus.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Name:        "pool_tasks_failed_total",
			Help:        "Number of tasks that completed with panic",
			ConstLabels: poolLabels,
		},
		func() float64 {
			return float64(pool.FailedTasks())
		}))
	prometheus.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Name:        "pool_tasks_completed_total",
			Help:        "Number of tasks that completed either successfully or with panic",
			ConstLabels: poolLabels,
		},
		func() float64 {
			return float64(pool.CompletedTasks())
		}))
}

func instrumentPool(name string, argoCDName string, pool pond.Pool) {
	poolLabels := make(map[string]string)
	poolLabels["pool"] = name
	poolLabels["argocd"] = argoCDName
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name:        "pool_workers_running",
			Help:        "Number of running worker goroutines",
			ConstLabels: poolLabels,
		},
		func() float64 {
			return float64(pool.RunningWorkers())
		}))
	// Task metrics
	prometheus.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Name:        "pool_tasks_submitted_total",
			Help:        "Number of tasks submitted",
			ConstLabels: poolLabels,
		},
		func() float64 {
			return float64(pool.SubmittedTasks())
		}))
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name:        "pool_tasks_waiting_total",
			Help:        "Number of tasks waiting in the queue",
			ConstLabels: poolLabels,
		},
		func() float64 {
			return float64(pool.WaitingTasks())
		}))
	prometheus.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Name:        "pool_tasks_successful_total",
			Help:        "Number of tasks that completed successfully",
			ConstLabels: poolLabels,
		},
		func() float64 {
			return float64(pool.SuccessfulTasks())
		}))
	prometheus.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Name:        "pool_tasks_failed_total",
			Help:        "Number of tasks that completed with panic",
			ConstLabels: poolLabels,
		},
		func() float64 {
			return float64(pool.FailedTasks())
		}))
	prometheus.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{
			Name:        "pool_tasks_completed_total",
			Help:        "Number of tasks that completed either successfully or with panic",
			ConstLabels: poolLabels,
		},
		func() float64 {
			return float64(pool.CompletedTasks())
		}))
}

// Labels for clientDialsTotal.
const (
	dialReasonInitial   = "initial"
	dialReasonReconnect = "reconnect"

	dialResultSuccess = "success"
	dialResultFailure = "failure"
)

// Labels for clientRetriesTotal.
const (
	// retryReasonTransient: the response was lost in transit (see
	// isTransientTransportFailure) and the call was repeated as-is.
	retryReasonTransient = "transient"
	// retryReasonReconnect: the connection itself was gone, so the call was
	// repeated on a freshly dialed one.
	retryReasonReconnect = "reconnect"
)

// Connection metrics are package-level vectors registered once per process,
// unlike the pool collectors above: those carry const labels and are registered
// per pool, which is why they're skipped under DoNotInstrument (a second
// registration with the same labels panics). One vector shared by every client
// has no such problem, so these are always on — and the reconnect counter is
// the signal that decides whether redialing needs rate-limiting, so it needs to
// be there in every environment. See
// docs/adrs/0025-reconnect-the-argocd-grpc-client.md.
var (
	clientDialsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_client_dials_total",
			Help: "Number of connections dialed to an ArgoCD instance, by why it was dialed and whether it worked",
		},
		[]string{"argocd", "reason", "result"},
	)

	clientConnectionGeneration = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argocd_client_connection_generation",
			Help: "Number of connections this process has established to an ArgoCD instance; increments on every reconnect",
		},
		[]string{"argocd"},
	)

	clientRetriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_client_retries_total",
			Help: "Number of ArgoCD RPCs repeated after a failure, by method and why the first attempt failed",
		},
		[]string{"argocd", "method", "reason"},
	)
)
