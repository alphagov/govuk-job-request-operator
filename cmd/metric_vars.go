package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	jobRequestReceivedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "job_request_total_received",
			Help: "Total number of job requests received",
		},
	)
	jobRequestRequeueTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_requeue_total",
			Help: "Total number of job requests that were requeued",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestSuccessfulReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_successful_reconcile_total",
			Help: "Total number of job requests that successfully reconciled",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestErrorGetTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_error_get_total",
			Help: "Total number of errors getting job requests",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestErrorAlreadyDeletedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_error_already_deleted_total",
			Help: "Total number of job requests that were already deleted",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestErrorDeletingTtlTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_error_deleting_ttl_total",
			Help: "Total number of errors deleting job requests due to TTL",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestDeletedByTtlTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_deleted_by_ttl_total",
			Help: "Total number of job requests deleted due to TTL",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestAlreadyInTerminalStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_already_in_terminal_state_total",
			Help: "Total number of job requests that were already in a terminal state",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestErrorRequestedByAnnoTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_error_requested_by_anno_total",
			Help: "Total number of errors validating the requested-by annotation",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestNoneFoundTargetResourceTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_none_found_target_resource_total",
			Help: "Total number of times target resource was not found",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestErrorCreateJobTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_error_create_job_total",
			Help: "Total number of errors creating the job",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestPendingStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_pending_state_total",
			Help: "Total number of job requests in pending state",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestApprovedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_approved_state_total",
			Help: "Total number of job requests in approved state",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestRejectedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_rejected_state_total",
			Help: "Total number of job requests in rejected state",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestStartedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_started_state_total",
			Help: "Total number of job requests in started state",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestMalformedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_malformed_state_total",
			Help: "Total number of job requests in malformed state",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestJobCompleteStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_job_complete_state_total",
			Help: "Total number of job requests in complete state",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestJobFailedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_job_failed_state_total",
			Help: "Total number of job requests in failed state",
		},
		[]string{"namespaced_name", "state"},
	)
	jobRequestTimeTilReview = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "job_request_time_til_review_seconds",
			Help: "Time until job request review",
		},
		[]string{"namespaced_name", "state"},
	)
)
