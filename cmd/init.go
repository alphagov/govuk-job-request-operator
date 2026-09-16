package main

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
	prommetrics "github.com/alphagov/govuk-job-request-operator/internal/metrics"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(platformv1.AddToScheme(scheme))

	metrics.Registry.MustRegister(
		prommetrics.JobRequestReceivedTotal,
		prommetrics.JobRequestRequeueTotal,
		prommetrics.JobRequestSuccessfulReconcileTotal,
		prommetrics.JobRequestErrorGettingRequestTotal,
		prommetrics.JobRequestErrorAlreadyDeletedTotal,
		prommetrics.JobRequestErrorDeletingByTtlTotal,
		prommetrics.JobRequestDeletedByTtlTotal,
		prommetrics.JobRequestAlreadyInTerminalStateTotal,
		prommetrics.JobRequestErrorRequestedByAnnoTotal,
		prommetrics.JobRequestNoneFoundTargetResourceTotal,
		prommetrics.JobRequestErrorCreateJobTotal,
		prommetrics.JobRequestPendingStateTotal,
		prommetrics.JobRequestApprovedStateTotal,
		prommetrics.JobRequestRejectedStateTotal,
		prommetrics.JobRequestStartedStateTotal,
		prommetrics.JobRequestMalformedStateTotal,
		prommetrics.JobRequestJobCompleteStateTotal,
		prommetrics.JobRequestJobFailedStateTotal,
		prommetrics.JobRequestTimeTilReview,
		prommetrics.JobRequestReviewReceivedTotal,
		prommetrics.JobRequestReviewRequeueTotal,
		prommetrics.JobRequestReviewErrorGettingReviewTotal,
		prommetrics.JobRequestReviewErrorAlreadyDeletedTotal,
		prommetrics.JobRequestReviewErrorDeletingByTtlTotal,
		prommetrics.JobRequestReviewDeletedByTtlTotal,
		prommetrics.JobRequestReviewAlreadyHasStateTotal,
		prommetrics.JobRequestReviewErrorReviewByAnnoTotal,
		prommetrics.JobRequestReviewErrorGettingRequestTotal,
		prommetrics.JobRequestReviewNoRequestFoundTotal,
		prommetrics.JobRequestReviewMalformedStateTotal,
		prommetrics.JobRequestReviewNotFoundStateTotal,
		prommetrics.JobRequestReviewConflictStateTotal,
		prommetrics.JobRequestReviewApprovedStateTotal,
		prommetrics.JobRequestReviewRejectedStateTotal,
		prommetrics.JobRequestReviewSuccessfulReconcileTotal,
	)

	// +kubebuilder:scaffold:scheme
}
