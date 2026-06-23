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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
)

// JobRequestReviewReconciler reconciles a JobRequestReview object
type JobRequestReviewReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequestreviews,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequestreviews/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequestreviews/finalizers,verbs=update

func (r *JobRequestReviewReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Attempt to retrieve JobRequestReview object and if not stop reconcile
	jobRequestReview := &platformv1.JobRequestReview{}
	err := r.Get(ctx, req.NamespacedName, jobRequestReview)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("JobRequestReview resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object
		log.Error(err, "Failed to get JobRequestReview")
		return ctrl.Result{}, nil
	}

	// Attempt to retrieve JobRequest object and if not stop reconcile and add something to the CRD to indicate a failure
	jobRequest := &platformv1.JobRequest{}
	// Compose namespace with jobRequest.Name instead of req.NamespacedName
	err = r.Get(ctx, req.NamespacedName, jobRequest)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Error(err, "JobRequest resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object
		log.Error(err, "Failed to get JobRequest")
		return ctrl.Result{}, nil
	}

	// If state is nil then reschedule as jobRequest object isn't setup yet
	if jobRequest.Status.State == "" {
		// Requeue the reconcile after certain time
		return ctrl.Result{RequeueAfter: time.Minute}, nil
	}

	// If state is failed then stop reconcile and add something to the JobRequestReview to indicate a failure
	if jobRequest.Status.State == "Failed" {
		// Create an error and log it
		log.Error(err, "JobRequest is in a Failed state so can't approve")
		return ctrl.Result{}, nil
	}

	jobRequest.Status.State = jobRequestReview.Spec.Decision
	// Do we need to recover the object again before flushing the state?
	r.Status().Update(ctx, jobRequest)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JobRequestReviewReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1.JobRequestReview{}).
		Named("jobrequestreview").
		Complete(r)
}
