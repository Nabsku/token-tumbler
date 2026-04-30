package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	TokenRotations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "token_tumbler_token_rotations_total",
		Help: "Total number of token rotation attempts",
	}, []string{"target_type", "repo_name", "secret_store", "outcome"})

	TokenRotationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "token_tumbler_token_rotation_duration_seconds",
		Help:    "Duration of token rotation operations in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"target_type", "repo_name"})

	SecretStoreOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "token_tumbler_secret_store_operations_total",
		Help: "Total number of secret store operations",
	}, []string{"secret_store", "operation", "outcome"})

	ActiveTokens = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "token_tumbler_active_tokens",
		Help: "Number of active tokens found per repository",
	}, []string{"target_type", "repo_name"})
)
