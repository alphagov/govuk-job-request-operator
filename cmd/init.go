package main

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(platformv1.AddToScheme(scheme))

	metrics.Registry.MustRegister(
		jobRequestReceivedTotal,
		jobRequestRequeueTotal,
		jobRequestSuccessfulReconcileTotal,
		jobRequestErrorGetTotal,
		jobRequestErrorAlreadyDeletedTotal,
		jobRequestErrorDeletingByTtlTotal,
		jobRequestDeletedByTtlTotal,
		jobRequestAlreadyInTerminalStateTotal,
		jobRequestErrorRequestedByAnnoTotal,
		jobRequestNoneFoundTargetResourceTotal,
		jobRequestErrorCreateJobTotal,
		jobRequestPendingStateTotal,
		jobRequestApprovedStateTotal,
		jobRequestRejectedStateTotal,
		jobRequestStartedStateTotal,
		jobRequestMalformedStateTotal,
		jobRequestJobCompleteStateTotal,
		jobRequestJobFailedStateTotal,
		jobRequestTimeTilReview,
		jobRequestReviewReceivedTotal,
		jobRequestReviewRequeueTotal,
		jobRequestReviewErrorGettingReviewTotal,
		jobRequestReviewErrorAlreadyDeletedTotal,
		jobRequestReviewErrorDeletingByTtlTotal,
		jobRequestReviewDeletedByTtlTotal,
		jobRequestReviewAlreadyHasStateTotal,
		jobRequestReviewErrorReviewByAnnoTotal,
		jobRequestReviewErrorGettingRequestTotal,
		jobRequestReviewNoRequestFoundTotal,
		jobRequestReviewMalformedStateTotal,
		jobRequestReviewNotFoundStateTotal,
		jobRequestReviewConflictStateTotal,
		jobRequestReviewApprovedStateTotal,
		jobRequestReviewRejectedStateTotal,
		jobRequestReviewSuccessfulReconcileTotal,
	)

	// +kubebuilder:scaffold:scheme
}
