package argocd

import (
	"fmt"

	"github.com/alitto/pond/v2"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultListPoolWorkers = 10
	defaultDiffPoolWorkers = 5
)

type IArgoCDClient interface{}

type ArgoCDClientOptions struct {
	ServerAddr      string
	Insecure        bool
	AuthToken       string
	ListPoolWorkers int
	DiffPoolWokers  int
}

type ArgoCDClient struct {
	ArgoCDClientOptions *ArgoCDClientOptions
	ListWorkerPool      pond.ResultPool[string]
	DiffWorkerPool      pond.ResultPool[string]
}

func instrumentWorkerPool(name string, pool pond.ResultPool[string]) {
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

func New(options *ArgoCDClientOptions) ArgoCDClient {
	if options.ListPoolWorkers == 0 {
		options.ListPoolWorkers = defaultListPoolWorkers
	}

	if options.DiffPoolWokers == 0 {
		options.DiffPoolWokers = defaultDiffPoolWorkers
	}

	client := ArgoCDClient{
		ArgoCDClientOptions: options,
	}

	client.ListWorkerPool = pond.NewResultPool[string](options.ListPoolWorkers)
	instrumentWorkerPool("list", client.ListWorkerPool)
	client.DiffWorkerPool = pond.NewResultPool[string](options.DiffPoolWokers)
	instrumentWorkerPool("diff", client.DiffWorkerPool)

	return client
}

func (a *ArgoCDClient) ListApplicationsByLabels(labels map[string]string) []string {
	group := a.ListWorkerPool.NewGroup()

	for key := range labels {
		value := labels[key]
		group.SubmitErr(func() (string, error) {
			return fmt.Sprintf("Value %s", value), nil
		})
	}

	results, _ := group.Wait()
	return results
}
