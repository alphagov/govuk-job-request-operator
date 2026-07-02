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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
	"github.com/go-logr/logr"
)

type JobRequestReviewReconciler struct {
	CacheClient     client.Client
	ApiServerClient client.Reader
	Scheme          *runtime.Scheme
	Log             logr.Logger
}

// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequestreviews,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequestreviews/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequestreviews/finalizers,verbs=update

func (r *JobRequestReviewReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	jobRequestReview := &platformv1.JobRequestReview{}

	err := r.getJobRequestReview(ctx, req.NamespacedName, jobRequestReview)
	if err != nil {
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

func (r *JobRequestReviewReconciler) getJobRequestReview(ctx context.Context, namespaceName client.ObjectKey, jobRequestReview *platformv1.JobRequestReview) error {
	err := r.CacheClient.Get(ctx, namespaceName, jobRequestReview)
	if apierrors.IsNotFound(err) {
		r.Log.Info("JobRequestReview resource not found. This is usually because the resource was deleted or not created. Ignoring and ending reconciliation")
		return err
	}

	if err != nil {
		r.Log.Error(err, "Failed to get JobRequestReview")
		return err
	}

	return nil
}

func (r *JobRequestReviewReconciler) getJobRequest(ctx context.Context, jobRequestReview *platformv1.JobRequestReview) (*ctrl.Result, *platformv1.JobRequest) {
	requestList := &platformv1.JobRequestList{}
	opts := []client.ListOption{
		client.MatchingFields{"metadata.name": jobRequestReview.Spec.JobRequestName},
	}

	if err := r.ApiServerClient.List(ctx, requestList, opts...); err != nil || len(requestList.Items) == 0 {
		jobRequestReview.Status.State = "JobRequestNotFound"
		err := r.CacheClient.Status().Update(ctx, jobRequestReview)
		if err != nil {
			r.Log.Error(err, "Failed to UPDATE JobRequestReview resource", "errored_obj", jobRequestReview)
		}

		r.Log.Error(err, "Failed to retrieve JobRequest")
		return &ctrl.Result{}, nil
	}

	return nil, &requestList.Items[0]
}

func (r *JobRequestReviewReconciler) handleReviewDecision(ctx context.Context, jobRequest *platformv1.JobRequest, jobRequestReview *platformv1.JobRequestReview) (ctrl.Result, error) {
	jobRequest.Status.State = jobRequestReview.Spec.Decision
	updateErr := r.CacheClient.Status().Update(ctx, jobRequest)
	if updateErr != nil {
		r.Log.Error(updateErr, "Failed to update status of JobRequest", "errored_obj", jobRequest)
	}

	if jobRequestReview.Status.State == "" {
		jobRequestReview.Status.State = jobRequestReview.Spec.Decision
		updateReviewErr := r.CacheClient.Status().Update(ctx, jobRequestReview)
		if updateReviewErr != nil {
			r.Log.Error(updateReviewErr, "Failed to update status of JobRequestReview", "errored_obj", jobRequestReview)
		}
	}

	return ctrl.Result{}, nil
}

func (r *JobRequestReviewReconciler) handleState(ctx context.Context, jobRequest *platformv1.JobRequest, jobRequestReview *platformv1.JobRequestReview) (ctrl.Result, error) {
	switch jobRequest.Status.State {
	case "":
		r.Log.Info("JobRequest hasn't finished creating so re-queueing the reconcile")
		return ctrl.Result{RequeueAfter: time.Minute}, nil

	case "Malformed":
		err := errors.New("JobRequest body Malformed")
		r.Log.Error(err, "JobRequest is in a Malformed state so can't approve")

		jobRequestReview.Status.State = "JobRequestMalformed"

		updateReviewErr := r.CacheClient.Status().Update(ctx, jobRequestReview)
		if updateReviewErr != nil {
			r.Log.Error(updateReviewErr, "Failed to update status of JobRequestReview", "errored_obj", jobRequestReview)
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
