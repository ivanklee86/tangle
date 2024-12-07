package argocd

import (
	"github.com/alitto/pond/v2"
	"github.com/prometheus/client_golang/prometheus"
)

func instrumentWorkers(name string, pool pond.ResultPool[string]) {
	poolLabels := make(map[string]string)
	poolLabels["pool"] = name
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
