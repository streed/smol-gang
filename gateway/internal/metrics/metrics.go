package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	WorkstreamsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smol_workstreams_total",
		Help: "Total workstreams created",
	}, []string{"status", "repository"})

	WorkstreamsActive = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "smol_workstreams_active",
		Help: "Currently active workstreams",
	})

	AgentMessagesSent = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smol_agent_messages_total",
		Help: "Total messages sent to/from agents",
	}, []string{"direction", "workstream_id"})

	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smol_http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "smol_http_request_duration_seconds",
		Help:    "HTTP request duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	PodProvisionDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "smol_pod_provision_duration_seconds",
		Help:    "Time to provision agent pods",
		Buckets: []float64{1, 5, 10, 30, 60, 120, 300},
	})

	LLMRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "smol_llm_requests_total",
		Help: "Total LLM API requests from agents",
	}, []string{"model", "status"})

	LLMRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "smol_llm_request_duration_seconds",
		Help:    "LLM API request duration",
		Buckets: []float64{0.5, 1, 2, 5, 10, 30, 60},
	}, []string{"model"})
)
