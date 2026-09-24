package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	RequestCurrentPendingTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "job_request_current_pending",
			Help: "Total number of job requests currently in pending state",
		},
	)
	RequestCurrentApprovedTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "job_request_current_approved",
			Help: "Total number of job requests currently in approved state",
		},
	)
	RequestCurrentRejectedTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "job_request_current_rejected",
			Help: "Total number of job requests currently in rejected state",
		},
	)
	RequestCurrentStartedTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "job_request_current_started",
			Help: "Total number of job requests currently in started state",
		},
	)
	RequestCurrentCompleteTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "job_request_current_complete",
			Help: "Total number of job requests currently in complete state",
		},
	)
	RequestCurrentFailedTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "job_request_current_failed",
			Help: "Total number of job requests currently in failed state",
		},
	)
	RequestCurrentMalformedTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "job_request_current_malformed",
			Help: "Total number of job requests currently in malformed state",
		},
	)
	ReviewCurrentApprovedTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "job_request_review_current_approved",
			Help: "Total number of job request reviews currently in approved state",
		},
	)
	ReviewCurrentRejectedTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "job_request_review_current_rejected",
			Help: "Total number of job request reviews currently in rejected state",
		},
	)
	ReviewCurrentMalformedTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "job_request_review_current_malformed",
			Help: "Total number of job request reviews currently in malformed state",
		},
	)
	ReviewCurrentNotFoundTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "job_request_review_current_not_found",
			Help: "Total number of job request reviews that currently have a job request which is 'not found' by the API server",
		},
	)
	ReviewCurrentConflictTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "job_request_review_current_conflict",
			Help: "Total number of job request reviews currently in conflict state",
		},
	)
)
