/*
MIT Licence

Copyright © 2013-2026 Crown Copyright (Government Digital Service)

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/

package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
)

type JobRequestReviewReconciler struct {
	CustomMetrics platformv1.ReviewCustomMetrics
	Reconciler[*platformv1.JobRequestReview]
}

// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequestreviews,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequestreviews/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequestreviews/finalizers,verbs=update

func (r *JobRequestReviewReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	jobRequestReview := &platformv1.JobRequestReview{}
	jobRequestReview.Name = req.Name
	jobRequestReview.Namespace = req.Namespace

	r.CustomMetrics.MetricLabels = prometheus.Labels{
		"namespaced_name": req.Namespace + "/" + req.Name,
		// nolint:goconst
		"state": "",
	}

	r.LogInfo("Received JobRequestReview.", jobRequestReview)
	r.CustomMetrics.ReceivedTotal.Inc()

	found := r.getJobRequestReview(ctx, req.NamespacedName, jobRequestReview)
	if !found {
		r.LogReconcillationExit(ReconcilliationFinished, "JobRequestReview not found.", jobRequestReview, nil)
		r.CustomMetrics.ErrorGettingReviewTotal.With(r.CustomMetrics.MetricLabels).Inc()
		return ctrl.Result{}, nil
	}

	r.CustomMetrics.MetricLabels["state"] = string(jobRequestReview.Status.State)

	age := time.Since(jobRequestReview.CreationTimestamp.Time)
	if age >= r.ResourceTtl {
		r.LogInfo("Pruning old JobRequestReview.", jobRequestReview, "age", age)
		err := r.CacheClient.Delete(ctx, jobRequestReview)
		if apierrors.IsNotFound(err) || apierrors.IsGone(err) {
			r.LogReconcillationExit(ReconcilliationFinished, "JobRequestReview is already deleted.", jobRequestReview, nil, "age", age)
			r.CustomMetrics.ErrorAlreadyDeletedTotal.With(r.CustomMetrics.MetricLabels).Inc()
			r.CustomMetrics.RequeueTotal.With(r.CustomMetrics.MetricLabels).Inc()
			return ctrl.Result{}, nil
		}
		if err != nil {
			r.LogReconcillationExit(ReconcilliationIncomplete,
				"Unexpected error when trying to delete JobRequestReview.",
				jobRequestReview, err, "age", age,
			)
			r.CustomMetrics.ErrorDeletingByTtlTotal.With(r.CustomMetrics.MetricLabels).Inc()
			r.CustomMetrics.RequeueTotal.With(r.CustomMetrics.MetricLabels).Inc()
			return ctrl.Result{}, err
		}

		r.LogReconcillationExit(ReconcilliationFinished, "JobRequestReview deleted after expired TTL.", jobRequestReview, nil, "age", age)
		r.CustomMetrics.DeletedByTtlTotal.With(r.CustomMetrics.MetricLabels).Inc()
		return ctrl.Result{}, nil
	}

	if jobRequestReview.Status.State != "" {
		r.LogReconcillationExit(ReconcilliationFinished,
			"JobRequestReview presented for reconcilliation, but it already has a state set.",
			jobRequestReview, nil,
			"state", jobRequestReview.Status.State,
		)
		r.CustomMetrics.AlreadyHasStateTotal.With(r.CustomMetrics.MetricLabels).Inc()
		return ctrl.Result{}, nil
	}

	if !r.validateReviewedByAnnotation(ctx, jobRequestReview) {
		reviewedByAnnotation, _ := jobRequestReview.GetReviewedBy()
		r.LogReconcillationExit(ReconcilliationFinished,
			"Could not validate reviewed-by annotation on JobRequestReview.",
			jobRequestReview, nil, "reviewedByAnnotation", reviewedByAnnotation,
		)
		r.CustomMetrics.ErrorReviewByAnnoTotal.With(r.CustomMetrics.MetricLabels).Inc()
		return ctrl.Result{}, nil
	}

	jobRequestList, err := r.getJobRequest(ctx, jobRequestReview)
	if err != nil {
		r.LogReconcillationExit(ReconcilliationIncomplete,
			"Error getting target JobRequest", jobRequestReview, err,
			"jobRequestReviewState", jobRequestReview.Status.State,
			"targetJobRequestName", jobRequestReview.Spec.JobRequestName,
		)
		r.CustomMetrics.ErrorGettingRequestTotal.With(r.CustomMetrics.MetricLabels).Inc()
		return ctrl.Result{}, err
	}
	if len(jobRequestList.Items) == 0 {
		r.LogReconcillationExit(ReconcilliationFinished,
			"Couldn't find the target JobRequest.", jobRequestReview, nil,
			"targetJobRequestName", jobRequestReview.Spec.JobRequestName,
		)
		r.CustomMetrics.NoRequestFoundTotal.With(r.CustomMetrics.MetricLabels).Inc()
		return ctrl.Result{}, nil
	}
	jobRequest := jobRequestList.Items[0]

	result, err := r.handleState(ctx, &jobRequest, jobRequestReview)
	if err != nil {
		r.LogReconcillationExit(ReconcilliationIncomplete,
			"Error handling state of JobRequestReview.",
			jobRequestReview, err,
		)
		r.CustomMetrics.RequeueTotal.With(r.CustomMetrics.MetricLabels).Inc()
		return result, err
	}

	r.LogReconcillationExit(ReconcilliationFinished, "Handled state of JobRequestReview succesfully.", jobRequestReview, nil)
	r.CustomMetrics.SuccessfulReconcileTotal.With(r.CustomMetrics.MetricLabels).Inc()
	return result, nil
}

func (r *JobRequestReviewReconciler) validateReviewedByAnnotation(ctx context.Context, jobRequestReview *platformv1.JobRequestReview) bool {
	reviewedBy, err := jobRequestReview.GetReviewedBy()
	if err != nil {
		r.LogError(err, "JobRequestReview missing reviewed-by annotation.", jobRequestReview)
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", err.Error())
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MetricLabels["state"] = string(platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MalformedStateTotal.With(r.CustomMetrics.MetricLabels).Inc()
		return false
	}

	_, err = platformv1.ParseUserIdentityFromARN(reviewedBy)
	if err != nil {
		r.LogError(
			err, "JobRequestReview has invalid reviewed-by field",
			jobRequestReview, "reviewedBy", reviewedBy,
		)
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", err.Error())
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MetricLabels["state"] = string(platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MalformedStateTotal.With(r.CustomMetrics.MetricLabels).Inc()
		return false
	}

	return true
}

func (r *JobRequestReviewReconciler) getJobRequestReview(ctx context.Context, namespaceName client.ObjectKey, jobRequestReview *platformv1.JobRequestReview) bool {
	err := r.CacheClient.Get(ctx, namespaceName, jobRequestReview)
	if err != nil {
		var errorLogMessage string
		if apierrors.IsNotFound(err) {
			errorLogMessage = "JobRequestReview not found. " +
				"This is usually because the resource was deleted or not created. " +
				"Ignoring error."
		} else {
			errorLogMessage = "Failed to deserialize JobRequestReview. Ignoring error."
		}
		r.LogError(err, errorLogMessage, jobRequestReview)
		return false
	}

	return true
}

func (r *JobRequestReviewReconciler) getJobRequest(ctx context.Context, jobRequestReview *platformv1.JobRequestReview) (platformv1.JobRequestList, error) {
	jobRequestList := platformv1.JobRequestList{}
	opts := []client.ListOption{
		client.MatchingFields{"metadata.name": jobRequestReview.Spec.JobRequestName},
		client.InNamespace(jobRequestReview.GetNamespace()),
	}

	err := r.ApiServerClient.List(ctx, &jobRequestList, opts...)
	if err != nil {
		r.LogError(
			err, "Error retrieving JobRequest to review.",
			jobRequestReview, "jobRequestName", jobRequestReview.Spec.JobRequestName,
		)
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, "Api Error", "None", fmt.Sprintf("Error when trying to list job requests from the API: %s", err.Error()))
		return jobRequestList, err
	}

	if len(jobRequestList.Items) == 0 {
		// State is already not found, no need to log anymore
		if jobRequestReview.Status.State == platformv1.JobRequestReviewNotFound {
			return jobRequestList, nil
		}

		r.LogInfo("JobRequest to review could not be found", jobRequestReview, "jobRequestName", jobRequestReview.Spec.JobRequestName)
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewNotFound), "None", "JobRequest could not be found")
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewNotFound)
		r.CustomMetrics.MetricLabels["state"] = string(platformv1.JobRequestReviewNotFound)
		r.CustomMetrics.NotFoundStateTotal.With(r.CustomMetrics.MetricLabels).Inc()
	}

	return jobRequestList, nil
}

func (r *JobRequestReviewReconciler) setState(ctx context.Context, jobRequestReview *platformv1.JobRequestReview, state platformv1.JobRequestReviewState) {
	jobRequestReview.Status.State = state
	err := r.CacheClient.Status().Update(ctx, jobRequestReview)
	if err != nil {
		r.LogError(err, "Failed to update state of JobRequestReview", jobRequestReview, "toState", string(state))
	}
}

func (r *JobRequestReviewReconciler) validateReviewerAndRequesterDiffer(ctx context.Context, jobRequest *platformv1.JobRequest, jobRequestReview *platformv1.JobRequestReview) error {
	requestedByAnnotation, err := jobRequest.GetRequestedBy()
	if err != nil {
		errorMessage := "error validating JobRequestReview reviewer and JobRequest requester differ. Unable to get requested-by annotation from the JobRequest"
		r.LogError(err, "Error validating reviewer and requester differ.", jobRequestReview)

		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", errorMessage)
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MetricLabels["state"] = string(platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MalformedStateTotal.With(r.CustomMetrics.MetricLabels).Inc()

		return errors.New(errorMessage)
	}

	requester, err := platformv1.ParseUserIdentityFromARN(requestedByAnnotation)
	if err != nil {
		errorMessage := "error validating JobRequestReview reviewer and JobRequest requester differ. Unable to parse requested-by annotation"
		r.LogError(err, "Error validating reviewer and requester differ.", jobRequestReview, "requestedByAnnotation", requestedByAnnotation)

		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", errorMessage)
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MetricLabels["state"] = string(platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MalformedStateTotal.With(r.CustomMetrics.MetricLabels).Inc()

		return errors.New(errorMessage)
	}

	reviewedByAnnotation, err := jobRequestReview.GetReviewedBy()
	if err != nil {
		errorMessage := "error validating JobRequestReview reviewer and JobRequest requester differ. Unable to get reviewed-by annotation from the JobRequestReview"
		r.LogError(err, "Error validating reviewer and requester differ.", jobRequestReview)

		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", errorMessage)
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MetricLabels["state"] = string(platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MalformedStateTotal.With(r.CustomMetrics.MetricLabels).Inc()

		return errors.New(errorMessage)
	}

	reviewer, err := platformv1.ParseUserIdentityFromARN(reviewedByAnnotation)
	if err != nil {
		errorMessage := "error validating JobRequestReview reviewer and JobRequest requester differ. Unable to parse reviewed-by annotation"
		r.LogError(err, "Error validating reviewer and requester differ.", jobRequestReview, "requestedByAnnotation", reviewedByAnnotation)

		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", errorMessage)
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MetricLabels["state"] = string(platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MalformedStateTotal.With(r.CustomMetrics.MetricLabels).Inc()

		return errors.New(errorMessage)
	}

	if reviewer.UserName == requester.UserName {
		errorMessage := "JobRequestReview reveiwer is the same as JobRequest requester. A user cannot review their own request"
		r.LogInfo(
			errorMessage, jobRequestReview,
			"jobRequestName", jobRequest.Name,
			"user", reviewer.UserName,
			"reviewedByAnnotation", reviewedByAnnotation,
			"requestedByAnnotation", requestedByAnnotation,
		)
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewConflict), "None", errorMessage)
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewConflict)
		r.CustomMetrics.MetricLabels["state"] = string(platformv1.JobRequestReviewConflict)
		r.CustomMetrics.ConflictStateTotal.With(r.CustomMetrics.MetricLabels).Inc()

		return errors.New(errorMessage)
	}

	return nil
}

func (r *JobRequestReviewReconciler) handleReviewDecision(ctx context.Context, jobRequest *platformv1.JobRequest, jobRequestReview *platformv1.JobRequestReview) (ctrl.Result, error) {
	err := r.validateReviewerAndRequesterDiffer(ctx, jobRequest, jobRequestReview)
	if err != nil {
		return ctrl.Result{}, err
	}

	jobRequest.Status.State = platformv1.JobRequestState(jobRequestReview.Spec.Decision)
	jobRequest.Status.ReviewName = jobRequestReview.Name

	r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeNormal, jobRequestReview.Spec.Decision, "None",
		"JobRequest is %s", jobRequestReview.Spec.Decision)

	updateErr := r.CacheClient.Status().Update(ctx, jobRequest)
	if updateErr != nil {
		r.LogError(updateErr,
			"Failed to update JobRequest to set review decision and name of the JobRequestReview.",
			jobRequestReview,
			"jobRequestName", jobRequest.Name,
			"reviewDecision", jobRequestReview.Spec.Decision,
		)
	}

	if jobRequestReview.Status.State == "" {
		switch jobRequestReview.Spec.Decision {
		case string(platformv1.JobRequestReviewApproved):
			r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewApproved)
			r.CustomMetrics.MetricLabels["state"] = string(platformv1.JobRequestReviewApproved)
			r.CustomMetrics.ApprovedStateTotal.With(r.CustomMetrics.MetricLabels).Inc()
		case string(platformv1.JobRequestReviewRejected):
			r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewRejected)
			r.CustomMetrics.MetricLabels["state"] = string(platformv1.JobRequestReviewRejected)
			r.CustomMetrics.RejectedStateTotal.With(r.CustomMetrics.MetricLabels).Inc()
		}

		r.LogInfo(
			"JobRequest has been reviewed", jobRequestReview,
			"jobRequestName", jobRequest.Name,
			"reviewDecision", jobRequestReview.Status.State,
		)
	}

	return ctrl.Result{}, nil
}

func (r *JobRequestReviewReconciler) handleState(ctx context.Context, jobRequest *platformv1.JobRequest, jobRequestReview *platformv1.JobRequestReview) (ctrl.Result, error) {
	switch jobRequest.Status.State {
	case "":
		r.LogInfo(
			"JobRequest has no state yet, re-queueing the reconcile.",
			jobRequestReview,
			"jobRequestName", jobRequest.Name,
		)
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeNormal, string(platformv1.JobRequestPending), "None", "JobRequest has no state yet")
		r.CustomMetrics.RequeueTotal.With(r.CustomMetrics.MetricLabels).Inc()
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil

	case platformv1.JobRequestMalformed:
		err := errors.New("JobRequest body Malformed")
		r.LogError(err,
			"JobRequest is in a Malformed state so can't approve",
			jobRequestReview,
			"jobRequestName", jobRequest.Name,
		)

		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", "JobRequest is in a Malformed state")
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MetricLabels["state"] = string(platformv1.JobRequestReviewMalformed)
		r.CustomMetrics.MalformedStateTotal.With(r.CustomMetrics.MetricLabels).Inc()

		return ctrl.Result{}, nil

	case platformv1.JobRequestConflicted:
		err := errors.New("JobRequest is Conflicted")
		r.LogError(err, "JobRequest is in a Conflicted state and cannot be reviewed.", jobRequestReview)

		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewConflict), "None", "JobRequest is in a Conflicted state")
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewConflict)
		r.CustomMetrics.MetricLabels["state"] = string(platformv1.JobRequestReviewConflict)
		r.CustomMetrics.ConflictStateTotal.With(r.CustomMetrics.MetricLabels).Inc()

		return ctrl.Result{}, nil

	case platformv1.JobRequestPending:
		return r.handleReviewDecision(ctx, jobRequest, jobRequestReview)

	case platformv1.JobRequestRejected, platformv1.JobRequestApproved, platformv1.JobRequestStarted, platformv1.JobRequestComplete, platformv1.JobRequestFailed:
		if jobRequest.WasReviewedBy(jobRequestReview) {
			return ctrl.Result{}, nil
		}

		errorMessage := fmt.Sprintf(
			"JobRequest already reviewed by JobRequestReview %s and is in state %s",
			jobRequest.Status.ReviewName,
			jobRequest.Status.State,
		)

		err := errors.New(errorMessage)

		r.LogError(err,
			"JobRequest has already been reviewed by another JobRequestReview.",
			jobRequestReview,
			"jobRequestName", jobRequest.Name,
			"jobRequestAlreadyReviewedBy", jobRequest.Status.ReviewName,
			"jobRequestState", jobRequest.Status.State,
		)

		r.Recorder.Eventf(
			jobRequestReview,
			nil,
			corev1.EventTypeWarning,
			string(platformv1.JobRequestReviewConflict),
			"None",
			errorMessage,
		)
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewConflict)
		r.CustomMetrics.MetricLabels["state"] = string(platformv1.JobRequestReviewConflict)
		r.CustomMetrics.ConflictStateTotal.With(r.CustomMetrics.MetricLabels).Inc()

		return ctrl.Result{}, nil
	default:
		err := fmt.Errorf("failed to reconcile JobRequestReview %s, JobRequest %s in an unknown state %s", jobRequestReview.Name, jobRequest.Name, jobRequest.Status.State)
		r.LogError(
			err,
			"JobRequestReview cannot currently be reconciled since the JobRequest is in an unknown state",
			jobRequestReview,
			"jobRequestName", jobRequest.Name,
			"jobRequestState", jobRequest.Status.State,
		)

		r.Recorder.Eventf(
			jobRequestReview,
			jobRequest,
			corev1.EventTypeWarning,
			"Unknown JobRequest State",
			"None",
			"JobRequest %s in an unknown state %s",
			jobRequest.Name,
			jobRequest.Status.State,
		)

		return ctrl.Result{}, err
	}
}

func (r *JobRequestReviewReconciler) SetupControllerWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1.JobRequestReview{}).
		Named("jobrequestreview").
		Complete(r)
}
