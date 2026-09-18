package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	vectorLabels = []string{"namespaced_name", "state"}

	jobRequestReceivedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "job_request_received_total",
			Help: "Total number of job requests received",
		},
	)
	jobRequestRequeueTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_requeue_total",
			Help: "Total number of job requests that were requeued",
		},
		vectorLabels,
	)
	jobRequestSuccessfulReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_successful_reconcile_total",
			Help: "Total number of job requests that successfully reconciled",
		},
		vectorLabels,
	)
	jobRequestErrorGetTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_error_get_total",
			Help: "Total number of errors getting job requests",
		},
		vectorLabels,
	)
	jobRequestErrorAlreadyDeletedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_error_already_deleted_total",
			Help: "Total number of job requests that were already deleted",
		},
		vectorLabels,
	)
	jobRequestErrorDeletingByTtlTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_error_deleting_ttl_total",
			Help: "Total number of errors deleting job requests due to TTL",
		},
		vectorLabels,
	)
	jobRequestDeletedByTtlTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_deleted_by_ttl_total",
			Help: "Total number of job requests deleted due to TTL",
		},
		vectorLabels,
	)
	jobRequestAlreadyInTerminalStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_already_in_terminal_state_total",
			Help: "Total number of job requests that were already in a terminal state",
		},
		vectorLabels,
	)
	jobRequestErrorRequestedByAnnoTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_error_requested_by_anno_total",
			Help: "Total number of errors validating the requested-by annotation",
		},
		vectorLabels,
	)
	jobRequestNoneFoundTargetResourceTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_none_found_target_resource_total",
			Help: "Total number of times target resource was not found",
		},
		vectorLabels,
	)
	jobRequestErrorCreateJobTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_error_create_job_total",
			Help: "Total number of errors creating the job",
		},
		vectorLabels,
	)
	jobRequestPendingStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_pending_state_total",
			Help: "Total number of job requests in pending state",
		},
		vectorLabels,
	)
	jobRequestApprovedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_approved_state_total",
			Help: "Total number of job requests in approved state",
		},
		vectorLabels,
	)
	jobRequestRejectedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_rejected_state_total",
			Help: "Total number of job requests in rejected state",
		},
		vectorLabels,
	)
	jobRequestStartedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_started_state_total",
			Help: "Total number of job requests in started state",
		},
		vectorLabels,
	)
	jobRequestMalformedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_malformed_state_total",
			Help: "Total number of job requests in malformed state",
		},
		vectorLabels,
	)
	jobRequestJobCompleteStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_job_complete_state_total",
			Help: "Total number of job requests in complete state",
		},
		vectorLabels,
	)
	jobRequestJobFailedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_job_failed_state_total",
			Help: "Total number of job requests in failed state",
		},
		vectorLabels,
	)
	jobRequestTimeTilReview = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "job_request_time_til_review_seconds",
			Help: "Time until job request review",
		},
		vectorLabels,
	)
)
