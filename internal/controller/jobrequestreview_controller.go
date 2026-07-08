/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"errors"
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

	if jobRequestReview.Status.State != "" {
		return ctrl.Result{}, nil
	}

	resourceResult, jobRequest := r.getJobRequest(ctx, jobRequestReview)
	if resourceResult != nil {
		return *resourceResult, nil
	}

	return r.handleState(ctx, jobRequest, jobRequestReview)
}

func (r *JobRequestReviewReconciler) getJobRequestReview(ctx context.Context, namespaceName client.ObjectKey, jobRequestReview *platformv1.JobRequestReview) bool {
	err := r.CacheClient.Get(ctx, namespaceName, jobRequestReview)
	if err != nil {
		var errorLogMessage string
		if apierrors.IsNotFound(err) {
			errorLogMessage = "JobRequestReview resource not found. This is usually because the resource was deleted or not created. Ignoring and ending reconciliation"
		} else {
			errorLogMessage = "Failed to deserialize JobRequest. Ignoring and ending reconciliation"
		}
		r.Log.Error(err, errorLogMessage)
		return false
	}

	return true
}

func (r *JobRequestReviewReconciler) getJobRequest(ctx context.Context, jobRequestReview *platformv1.JobRequestReview) (*ctrl.Result, *platformv1.JobRequest) {
	requestList := &platformv1.JobRequestList{}
	opts := []client.ListOption{
		client.MatchingFields{"metadata.name": jobRequestReview.Spec.JobRequestName},
	}

	if err := r.ApiServerClient.List(ctx, requestList, opts...); err != nil || len(requestList.Items) == 0 {
		jobRequestReview.Status.State = "JobRequestNotFound"
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, "JobRequestNotFound", "None", "JobRequest could not be found")
		err := r.CacheClient.Status().Update(ctx, jobRequestReview)
		if err != nil {
			r.Log.Error(err, "Failed to update status of JobRequestReview resource")
		}

		r.Log.Error(err, "Failed to retrieve JobRequest")
		return &ctrl.Result{}, nil
	}

	return nil, &requestList.Items[0]
}

func (r *JobRequestReviewReconciler) handleReviewDecision(ctx context.Context, jobRequest *platformv1.JobRequest, jobRequestReview *platformv1.JobRequestReview) (ctrl.Result, error) {
	jobRequest.Status.State = jobRequestReview.Spec.Decision
	r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeNormal, jobRequestReview.Spec.Decision, "None",
		"JobRequest is %s", jobRequestReview.Spec.Decision)
	updateErr := r.CacheClient.Status().Update(ctx, jobRequest)
	if updateErr != nil {
		r.Log.Error(updateErr, "Failed to update status of JobRequest")
	}

	if jobRequestReview.Status.State == "" {
		jobRequestReview.Status.State = jobRequestReview.Spec.Decision
		updateReviewErr := r.CacheClient.Status().Update(ctx, jobRequestReview)
		if updateReviewErr != nil {
			r.Log.Error(updateReviewErr, "Failed to update status of JobRequestReview")
		}
	}

	return ctrl.Result{}, nil
}

func (r *JobRequestReviewReconciler) handleState(ctx context.Context, jobRequest *platformv1.JobRequest, jobRequestReview *platformv1.JobRequestReview) (ctrl.Result, error) {
	switch jobRequest.Status.State {
	case "":
		r.Log.Info("JobRequest hasn't finished creating so re-queueing the reconcile")
		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeNormal, "Pending", "None", "JobRequest hasn't finished creating")
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil

	case "Malformed":
		err := errors.New("JobRequest body Malformed")
		r.Log.Error(err, "JobRequest is in a Malformed state so can't approve")

		jobRequestReview.Status.State = "JobRequestMalformed"

		r.Recorder.Eventf(jobRequestReview, nil, corev1.EventTypeWarning, "JobRequestMalformed", "None", "JobRequest is in a Malformed state")
		updateReviewErr := r.CacheClient.Status().Update(ctx, jobRequestReview)
		if updateReviewErr != nil {
			r.Log.Error(updateReviewErr, "Failed to update status of JobRequestReview")
		}

		return ctrl.Result{}, nil

	case "Pending":
		jobRequest.Status.State = jobRequestReview.Spec.Decision
		jobRequest.Status.ReviewName = jobRequestReview.GetName()

		return r.handleReviewDecision(ctx, jobRequest, jobRequestReview)

	case "Approved", "Rejected", "Started", "Failed", "Completed":
		return ctrl.Result{}, nil

	default:
		return ctrl.Result{}, nil
	}
}

func (r *JobRequestReviewReconciler) SetupControllerWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1.JobRequestReview{}).
		Named("jobrequestreview").
		Complete(r)
}
