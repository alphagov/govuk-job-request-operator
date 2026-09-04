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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
	"github.com/go-logr/logr"
)

type JobRequestReviewReconciler struct {
	CacheClient     client.Client
	ApiServerClient client.Reader
	Scheme          *runtime.Scheme
	Recorder        events.EventRecorder
	Log             logr.Logger
	ResourceTtl     time.Duration
}

// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequestreviews,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequestreviews/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequestreviews/finalizers,verbs=update

func (r *JobRequestReviewReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	jobRequestReview := &platformv1.JobRequestReview{}

	found := r.getJobRequestReview(ctx, req.NamespacedName, jobRequestReview)
	if !found {
		return ctrl.Result{}, nil
	}

	age := time.Since(jobRequestReview.CreationTimestamp.Time)
	if age >= r.ResourceTtl {
		r.Log.Info("Pruning old JobRequestReview", "name", jobRequestReview.Name, "namespace", jobRequestReview.Namespace, "age", age)
		errMaybeNil := r.CacheClient.Delete(ctx, jobRequestReview)
		if apierrors.IsNotFound(errMaybeNil) || apierrors.IsGone(errMaybeNil) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errMaybeNil
	}

	if jobRequestReview.Status.State != "" {
		r.Log.Info(
			fmt.Sprintf(
				"JobRequestReview %s presented for reconcilliation, but it already has State %s, no further reconcilliation will happen",
				jobRequestReview.Name,
				jobRequestReview.Status.State,
			),
		)

		return ctrl.Result{}, nil
	}

	if !r.validateReviewedByAnnotation(ctx, jobRequestReview) {
		return ctrl.Result{}, nil
	}

	jobRequestList, err := r.getJobRequest(ctx, jobRequestReview)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(jobRequestList.Items) == 0 {
		return ctrl.Result{}, nil
	}
	jobRequest := jobRequestList.Items[0]

	return r.handleState(ctx, &jobRequest, jobRequestReview)
}

func (r *JobRequestReviewReconciler) validateReviewedByAnnotation(ctx context.Context, jobRequestReview *platformv1.JobRequestReview) bool {
	reviewedBy, err := jobRequestReview.GetReviewedBy()

	if err != nil {
		r.Log.Error(err, "Missing reviewed-by field")
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", err.Error())
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)
		return false
	}

	_, err = platformv1.ParseUserIdentityFromARN(reviewedBy)
	if err != nil {
		r.Log.Error(err, "Invalid reviewed-by field")
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", err.Error())
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)
		return false
	}

	return true
}

func (r *JobRequestReviewReconciler) getJobRequestReview(ctx context.Context, namespaceName client.ObjectKey, jobRequestReview *platformv1.JobRequestReview) bool {
	err := r.CacheClient.Get(ctx, namespaceName, jobRequestReview)
	if err != nil {
		var errorLogMessage string
		if apierrors.IsNotFound(err) {
			errorLogMessage = "JobRequestReview resource not found. This is usually because the resource was deleted or not created. Ignoring and ending reconciliation"
		} else {
			errorLogMessage = "Failed to deserialize JobRequestReview. Ignoring and ending reconciliation"
		}
		r.Log.Error(err, errorLogMessage)
		return false
	}

	return true
}

func (r *JobRequestReviewReconciler) getJobRequest(ctx context.Context, jobRequestReview *platformv1.JobRequestReview) (platformv1.JobRequestList, error) {
	jobRequestList := platformv1.JobRequestList{}
	opts := []client.ListOption{
		client.MatchingFields{"metadata.name": jobRequestReview.Spec.JobRequestName},
	}

	err := r.ApiServerClient.List(ctx, &jobRequestList, opts...)
	if err != nil {
		r.Log.Error(err, "Failed to retrieve JobRequest to review")
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, "Api Error", "None", fmt.Sprintf("Error when trying to list job requests from the API: %s", err.Error()))
		return jobRequestList, err
	}

	if len(jobRequestList.Items) == 0 {
		// State is already not found, no need to log anymore
		if jobRequestReview.Status.State == platformv1.JobRequestReviewNotFound {
			return jobRequestList, nil
		}

		err := fmt.Errorf("job request %s could not be found", jobRequestReview.Spec.JobRequestName)
		r.Log.Error(err, "Failed to retrieve JobRequest")
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewNotFound), "None", "JobRequest could not be found")
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewNotFound)
	}

	return jobRequestList, nil
}

func (r *JobRequestReviewReconciler) setState(ctx context.Context, jobRequestReview *platformv1.JobRequestReview, state platformv1.JobRequestReviewState) {
	jobRequestReview.Status.State = state
	err := r.CacheClient.Status().Update(ctx, jobRequestReview)
	if err != nil {
		r.Log.Error(err, fmt.Sprintf("Failed to update state of JobRequestReview %s to %s", jobRequestReview.Name, state))
	}
}

func (r *JobRequestReviewReconciler) validateReviewerAndRequesterDiffer(ctx context.Context, jobRequest *platformv1.JobRequest, jobRequestReview *platformv1.JobRequestReview) error {
	requestedByAnnotation, err := jobRequest.GetRequestedBy()
	if err != nil {
		errorMessage := fmt.Sprintf("Error validating reviewer and requester differ. Unable to get requested-by annotation from the JobRequest. Error: %s", err.Error())
		r.Log.Error(err, errorMessage)

		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", errorMessage)
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)

		return errors.New(errorMessage)
	}

	requester, err := platformv1.ParseUserIdentityFromARN(requestedByAnnotation)
	if err != nil {
		errorMessage := fmt.Sprintf("Error validating reviewer and requester differ. Unable to parse JobRequest requested-by annotation. Error: %s", err.Error())
		r.Log.Error(err, errorMessage)

		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", errorMessage)
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)

		return errors.New(errorMessage)
	}

	reviewedByAnnotation, err := jobRequestReview.GetReviewedBy()
	if err != nil {
		errorMessage := fmt.Sprintf("Error validating reviewer and requester differ. Unable to get reviewed-by annotation from the JobRequestReview. Error: %s", err.Error())
		r.Log.Error(err, errorMessage)

		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", errorMessage)
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)

		return errors.New(errorMessage)
	}

	reviewer, err := platformv1.ParseUserIdentityFromARN(reviewedByAnnotation)
	if err != nil {
		errorMessage := fmt.Sprintf("error validating reviewer and requester differ. Unable to parse JobRequestReview reviewed-by annotation. Error: %s", err.Error())
		r.Log.Error(err, errorMessage)

		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", errorMessage)
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)

		return errors.New(errorMessage)
	}

	if reviewer.UserName == requester.UserName {
		errorMessage := "the JobRequest cannot be reviewed by a JobRequestReview that was created by the same user as the original JobRequest"
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewConflict), "None", errorMessage)
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewConflict)

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
		r.Log.Error(updateErr, fmt.Sprintf("Failed to update state of JobRequest %s to %s and its review name to %s", jobRequest.Name, jobRequest.Status.State, jobRequestReview.Name))
	}

	if jobRequestReview.Status.State == "" {
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewState(jobRequestReview.Spec.Decision))
	}

	return ctrl.Result{}, nil
}

func (r *JobRequestReviewReconciler) handleState(ctx context.Context, jobRequest *platformv1.JobRequest, jobRequestReview *platformv1.JobRequestReview) (ctrl.Result, error) {
	switch jobRequest.Status.State {
	case "":
		r.Log.Info("JobRequest hasn't finished creating so re-queueing the reconcile")
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeNormal, string(platformv1.JobRequestPending), "None", "JobRequest hasn't finished creating")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil

	case platformv1.JobRequestMalformed:
		err := errors.New("JobRequest body Malformed")
		r.Log.Error(err, "JobRequest is in a Malformed state so can't approve")

		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, string(platformv1.JobRequestReviewMalformed), "None", "JobRequest is in a Malformed state")
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewMalformed)

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

		r.Log.Error(err, "JobRequest already reviewed")
		r.Recorder.Eventf(
			jobRequestReview,
			nil,
			corev1.EventTypeWarning,
			string(platformv1.JobRequestReviewConflict),
			"None",
			errorMessage,
		)
		r.setState(ctx, jobRequestReview, platformv1.JobRequestReviewConflict)

		return ctrl.Result{}, nil
	default:
		err := fmt.Errorf("failed to reconcile JobRequestReview %s, JobRequest %s in an unknown state %s", jobRequestReview.Name, jobRequest.Name, jobRequest.Status.State)
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
