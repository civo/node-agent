package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HealthCheckTotal counts the number of health check executions per node,
	// checker, and result (pass/fail).
	HealthCheckTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "civo_node_agent_health_check_total",
			Help: "Total number of health check executions.",
		},
		[]string{"node", "checker", "result"},
	)

	// RecoveryActionsTotal counts the number of recovery actions performed
	// per node, action type (reboot), and mode (report/active).
	RecoveryActionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "civo_node_agent_recovery_actions_total",
			Help: "Total number of recovery actions performed.",
		},
		[]string{"node", "action", "mode"},
	)

	// RecoveryFailuresTotal counts the number of recovery actions that failed
	// (e.g. Civo API errors).
	RecoveryFailuresTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "civo_node_agent_recovery_failures_total",
			Help: "Total number of recovery actions that failed.",
		},
		[]string{"node", "action"},
	)

	// NodeUnhealthyDurationSeconds tracks how long each node has been
	// continuously unhealthy, in seconds.
	NodeUnhealthyDurationSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "civo_node_agent_node_unhealthy_duration_seconds",
			Help: "Duration in seconds a node has been continuously unhealthy.",
		},
		[]string{"node"},
	)

	// RecoveryPhase reports the current recovery phase for each node.
	// The value is the numeric NodePhase (0=Healthy, 1=Unhealthy, etc.).
	RecoveryPhase = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "civo_node_agent_recovery_phase",
			Help: "Current recovery phase of a node.",
		},
		[]string{"node", "phase"},
	)
)

// Register registers all node-agent metrics with the default Prometheus registerer.
func Register() {
	prometheus.MustRegister(
		HealthCheckTotal,
		RecoveryActionsTotal,
		RecoveryFailuresTotal,
		NodeUnhealthyDurationSeconds,
		RecoveryPhase,
	)
}

// Handler returns an http.Handler that serves Prometheus metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}
