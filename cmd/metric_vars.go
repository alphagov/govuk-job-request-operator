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
			Name: "job_request_get_error_total",
			Help: "Total number of errors getting job requests",
		},
		vectorLabels,
	)
	jobRequestErrorAlreadyDeletedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_already_deleted_error_total",
			Help: "Total number of job requests that were already deleted",
		},
		vectorLabels,
	)
	jobRequestErrorDeletingByTtlTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_deleting_ttl_error_total",
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
			Name: "job_request_requested_by_anno_error_total",
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
			Name: "job_request_create_job_error_total",
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
	jobRequestReviewReceivedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "job_request_review_received_total",
			Help: "Total number of job request reviews received",
		},
	)
	jobRequestReviewRequeueTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_requeue_total",
			Help: "Total number of job request reviews that were requeued",
		},
		vectorLabels,
	)
	jobRequestReviewErrorGettingReviewTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_getting_review_error_total",
			Help: "Total number of errors getting job request reviews",
		},
		vectorLabels,
	)
	jobRequestReviewErrorAlreadyDeletedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_already_deleted_error_total",
			Help: "Total number of job request reviews that were already deleted",
		},
		vectorLabels,
	)
	jobRequestReviewErrorDeletingByTtlTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_deleting_by_ttl_error_total",
			Help: "Total number of errors deleting job request reviews due to TTL",
		},
		vectorLabels,
	)
	jobRequestReviewDeletedByTtlTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_deleted_by_ttl_total",
			Help: "Total number of job request reviews deleted due to TTL",
		},
		vectorLabels,
	)
	jobRequestReviewAlreadyHasStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_already_has_state_total",
			Help: "Total number of job request reviews that already have a state",
		},
		vectorLabels,
	)
	jobRequestReviewErrorReviewByAnnoTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_by_anno_error_total",
			Help: "Total number of errors validating the reviewed-by annotation",
		},
		vectorLabels,
	)
	jobRequestReviewErrorGettingRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_getting_request_error_total",
			Help: "Total number of errors getting the target job request",
		},
		vectorLabels,
	)
	jobRequestReviewNoRequestFoundTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_no_request_found_total",
			Help: "Total number of times the target job request was not found",
		},
		vectorLabels,
	)
	jobRequestReviewMalformedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_malformed_state_total",
			Help: "Total number of job request reviews in malformed state",
		},
		vectorLabels,
	)
	jobRequestReviewNotFoundStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_not_found_state_total",
			Help: "Total number of job request reviews in not found state",
		},
		vectorLabels,
	)
	jobRequestReviewConflictStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_conflict_state_total",
			Help: "Total number of job request reviews in conflict state",
		},
		vectorLabels,
	)
	jobRequestReviewApprovedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_approved_state_total",
			Help: "Total number of job request reviews in approved state",
		},
		vectorLabels,
	)
	jobRequestReviewRejectedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_rejected_state_total",
			Help: "Total number of job request reviews in rejected state",
		},
		vectorLabels,
	)
	jobRequestReviewSuccessfulReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_successful_reconcile_total",
			Help: "Total number of job request reviews that successfully reconciled",
		},
		vectorLabels,
	)
)
