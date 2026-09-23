package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	vectorLabels = []string{"namespaced_name", "state"}

	JobRequestReceivedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "job_request_received_total",
			Help: "Total number of job requests received",
		},
	)
	JobRequestRequeueTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_requeue_total",
			Help: "Total number of job requests that were requeued",
		},
		vectorLabels,
	)
	JobRequestSuccessfulReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_successful_reconcile_total",
			Help: "Total number of job requests that successfully reconciled",
		},
		vectorLabels,
	)
	JobRequestErrorGettingRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_get_error_total",
			Help: "Total number of errors getting job requests",
		},
		vectorLabels,
	)
	JobRequestErrorAlreadyDeletedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_already_deleted_error_total",
			Help: "Total number of job requests that were already deleted",
		},
		vectorLabels,
	)
	JobRequestErrorDeletingByTtlTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_deleting_ttl_error_total",
			Help: "Total number of errors deleting job requests due to TTL",
		},
		vectorLabels,
	)
	JobRequestDeletedByTtlTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_deleted_by_ttl_total",
			Help: "Total number of job requests deleted due to TTL",
		},
		vectorLabels,
	)
	JobRequestAlreadyInTerminalStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_already_in_terminal_state_total",
			Help: "Total number of job requests that were already in a terminal state",
		},
		vectorLabels,
	)
	JobRequestErrorRequestedByAnnoTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_requested_by_anno_error_total",
			Help: "Total number of errors validating the requested-by annotation",
		},
		vectorLabels,
	)
	JobRequestNoneFoundTargetResourceTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_none_found_target_resource_total",
			Help: "Total number of times target resource was not found",
		},
		vectorLabels,
	)
	JobRequestErrorCreateJobTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_create_job_error_total",
			Help: "Total number of errors creating the job",
		},
		vectorLabels,
	)
	JobRequestPendingStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_pending_state_total",
			Help: "Total number of job requests in pending state",
		},
		vectorLabels,
	)
	JobRequestApprovedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_approved_state_total",
			Help: "Total number of job requests in approved state",
		},
		vectorLabels,
	)
	JobRequestRejectedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_rejected_state_total",
			Help: "Total number of job requests in rejected state",
		},
		vectorLabels,
	)
	JobRequestStartedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_started_state_total",
			Help: "Total number of job requests in started state",
		},
		vectorLabels,
	)
	JobRequestMalformedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_malformed_state_total",
			Help: "Total number of job requests in malformed state",
		},
		vectorLabels,
	)
	JobRequestJobCompleteStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_job_complete_state_total",
			Help: "Total number of job requests in complete state",
		},
		vectorLabels,
	)
	JobRequestJobFailedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_job_failed_state_total",
			Help: "Total number of job requests in failed state",
		},
		vectorLabels,
	)
	JobRequestTimeTilReview = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "job_request_time_til_review_seconds",
			Help: "Time until job request review",
		},
		vectorLabels,
	)
	JobRequestReviewReceivedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "job_request_review_received_total",
			Help: "Total number of job request reviews received",
		},
	)
	JobRequestReviewRequeueTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_requeue_total",
			Help: "Total number of job request reviews that were requeued",
		},
		vectorLabels,
	)
	JobRequestReviewErrorGettingReviewTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_getting_review_error_total",
			Help: "Total number of errors getting job request reviews",
		},
		vectorLabels,
	)
	JobRequestReviewErrorAlreadyDeletedTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_already_deleted_error_total",
			Help: "Total number of job request reviews that were already deleted",
		},
		vectorLabels,
	)
	JobRequestReviewErrorDeletingByTtlTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_deleting_by_ttl_error_total",
			Help: "Total number of errors deleting job request reviews due to TTL",
		},
		vectorLabels,
	)
	JobRequestReviewDeletedByTtlTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_deleted_by_ttl_total",
			Help: "Total number of job request reviews deleted due to TTL",
		},
		vectorLabels,
	)
	JobRequestReviewAlreadyHasStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_already_has_state_total",
			Help: "Total number of job request reviews that already have a state",
		},
		vectorLabels,
	)
	JobRequestReviewErrorReviewByAnnoTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_by_anno_error_total",
			Help: "Total number of errors validating the reviewed-by annotation",
		},
		vectorLabels,
	)
	JobRequestReviewErrorGettingRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_getting_request_error_total",
			Help: "Total number of errors getting the target job request",
		},
		vectorLabels,
	)
	JobRequestReviewNoRequestFoundTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_no_request_found_total",
			Help: "Total number of times the target job request was not found",
		},
		vectorLabels,
	)
	JobRequestReviewMalformedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_malformed_state_total",
			Help: "Total number of job request reviews in malformed state",
		},
		vectorLabels,
	)
	JobRequestReviewNotFoundStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_not_found_state_total",
			Help: "Total number of job request reviews in not found state",
		},
		vectorLabels,
	)
	JobRequestReviewConflictStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_conflict_state_total",
			Help: "Total number of job request reviews in conflict state",
		},
		vectorLabels,
	)
	JobRequestReviewApprovedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_approved_state_total",
			Help: "Total number of job request reviews in approved state",
		},
		vectorLabels,
	)
	JobRequestReviewRejectedStateTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_rejected_state_total",
			Help: "Total number of job request reviews in rejected state",
		},
		vectorLabels,
	)
	JobRequestReviewSuccessfulReconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "job_request_review_successful_reconcile_total",
			Help: "Total number of job request reviews that successfully reconciled",
		},
		vectorLabels,
	)
)
