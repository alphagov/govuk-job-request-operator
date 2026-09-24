package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
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
