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
	// "maps"

	appsv1 "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// JobRequestReconciler reconciles a JobRequest object
type JobRequestReconciler struct {
	CacheClient     client.Client
	ApiServerClient client.Reader
	Scheme          *runtime.Scheme
}

func (r *JobRequestReconciler) CreateJobTemplate(resource *appsv1.Deployment) (*batch.Job, error) {
	// We want job names for a given nominal start time to have a deterministic name to avoid the same job being created twice
	// name := fmt.Sprintf("%s-%d", cronJob.Name, scheduledTime.Unix())

	jobTemplatePodSpec := *resource.Spec.Template.DeepCopy()
	jobTemplatePodSpec.Spec.RestartPolicy = "Never"

	job := &batch.Job{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
			Name:        resource.Name,
			Namespace:   resource.Namespace,
		},
		Spec: batch.JobSpec{
			Template: jobTemplatePodSpec,
		},
	}

	// maps.Copy(job.Annotations, resource.Spec)
	// job.Annotations[scheduledTimeAnnotation] = scheduledTime.Format(time.RFC3339)
	// maps.Copy(job.Labels, resource.Spec.JobTemplate.Labels)
	if err := ctrl.SetControllerReference(resource, job, r.Scheme); err != nil {
		return nil, err
	}

	return job, nil
}

// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequests/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the JobRequest object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.3/pkg/reconcile
func (r *JobRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Fetch the JobRequest instance
	jobRequest := &platformv1.JobRequest{}
	err := r.CacheClient.Get(ctx, req.NamespacedName, jobRequest)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("JobRequest resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get JobRequest")
		return ctrl.Result{}, err
	}

	/*
		Create a Job off from the jobrequest:
		Retrieving the deployment
		Create a Job from the deployment
		Run it
	*/

	deploymentList := &appsv1.DeploymentList{} // TODO: this could be another resource like another Job
	opts := []client.ListOption{
		client.MatchingFields{"metadata.name": req.Name},
	}

	if err := r.ApiServerClient.List(ctx, deploymentList, opts...); err != nil {
		log.Error(err, "Failed to list Resources") // TODO: this validation should be done in the cli / gatekeeper because a deployment that doesn't exist can't be used to create a job
		return ctrl.Result{}, nil
	}

	jobTemplate, err := r.CreateJobTemplate(&deploymentList.Items[0])
	if err != nil {
		log.Error(err, "Failed to create Job Template")
		return ctrl.Result{}, nil
	}

	createJobErr := r.CacheClient.Create(ctx, jobTemplate)
	if createJobErr != nil {
		log.Error(createJobErr, "Failed to create Job resource")
		return ctrl.Result{}, createJobErr
	}

	jobList := &batch.JobList{}
	if err := r.ApiServerClient.List(ctx, jobList, opts...); err != nil {
		log.Error(err, "Failed to list Resources")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *JobRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1.JobRequest{}).
		Named("jobrequest").
		Complete(r)
}
