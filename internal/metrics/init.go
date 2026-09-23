package metrics

import platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"

func InitRequestCustomMetrics() platformv1.RequestCustomMetrics {
	return platformv1.RequestCustomMetrics{
		ReceivedTotal:                JobRequestReceivedTotal,
		RequeueTotal:                 JobRequestRequeueTotal,
		SuccessfulReconcileTotal:     JobRequestSuccessfulReconcileTotal,
		ErrorGettingRequestTotal:     JobRequestErrorGettingRequestTotal,
		ErrorAlreadyDeletedTotal:     JobRequestErrorAlreadyDeletedTotal,
		ErrorDeletingByTtlTotal:      JobRequestErrorDeletingByTtlTotal,
		DeletedByTtlTotal:            JobRequestDeletedByTtlTotal,
		AlreadyInTerminalStateTotal:  JobRequestAlreadyInTerminalStateTotal,
		ErrorRequestedByAnnoTotal:    JobRequestErrorRequestedByAnnoTotal,
		NoneFoundTargetResourceTotal: JobRequestNoneFoundTargetResourceTotal,
		ErrorCreateJobTotal:          JobRequestErrorCreateJobTotal,
		PendingStateTotal:            JobRequestPendingStateTotal,
		ApprovedStateTotal:           JobRequestApprovedStateTotal,
		RejectedStateTotal:           JobRequestRejectedStateTotal,
		StartedStateTotal:            JobRequestStartedStateTotal,
		MalformedStateTotal:          JobRequestMalformedStateTotal,
		JobCompleteStateTotal:        JobRequestJobCompleteStateTotal,
		JobFailedStateTotal:          JobRequestJobFailedStateTotal,
		TimeTilReview:                JobRequestTimeTilReview,
	}
}

func InitReviewCustomMetrics() platformv1.ReviewCustomMetrics {
	return platformv1.ReviewCustomMetrics{
		ReceivedTotal:            JobRequestReviewReceivedTotal,
		RequeueTotal:             JobRequestReviewRequeueTotal,
		ErrorGettingReviewTotal:  JobRequestReviewErrorGettingReviewTotal,
		ErrorAlreadyDeletedTotal: JobRequestReviewErrorAlreadyDeletedTotal,
		ErrorDeletingByTtlTotal:  JobRequestReviewErrorDeletingByTtlTotal,
		DeletedByTtlTotal:        JobRequestReviewDeletedByTtlTotal,
		AlreadyHasStateTotal:     JobRequestReviewAlreadyHasStateTotal,
		ErrorReviewByAnnoTotal:   JobRequestReviewErrorReviewByAnnoTotal,
		ErrorGettingRequestTotal: JobRequestReviewErrorGettingRequestTotal,
		NoRequestFoundTotal:      JobRequestReviewNoRequestFoundTotal,
		MalformedStateTotal:      JobRequestReviewMalformedStateTotal,
		NotFoundStateTotal:       JobRequestReviewNotFoundStateTotal,
		ConflictStateTotal:       JobRequestReviewConflictStateTotal,
		ApprovedStateTotal:       JobRequestReviewApprovedStateTotal,
		RejectedStateTotal:       JobRequestReviewRejectedStateTotal,
		SuccessfulReconcileTotal: JobRequestReviewSuccessfulReconcileTotal,
	}
}
