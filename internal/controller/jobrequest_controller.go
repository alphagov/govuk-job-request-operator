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
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"

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

	jobRequest := &platformv1.JobRequest{}
	err := r.CacheClient.Get(ctx, req.NamespacedName, jobRequest)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the custom resource is not found then it usually means that it was deleted or not created
			// In this way, we will stop the reconciliation
			log.Info("JobRequest resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object
		log.Error(err, "Failed to get JobRequest")
		return ctrl.Result{}, nil
	}

	/*
		If status is nil then:
		Set state as pending
		Set jobName as nil
		Set requestedBy to "standard user"

		If pending then requeue again (consider rewording the job requests states in the CRD)
	*/

	/*
		If state is APPROVED then continue
		If state is DISAPPROVED then stop(?)
	*/

	// TODO: this could be another resource like another Job
	deploymentList := &appsv1.DeploymentList{}
	opts := []client.ListOption{
		client.MatchingFields{"metadata.name": req.Name},
	}

	// TODO: this validation should be done in the cli / gatekeeper because a deployment with valid values (e.g. container name and metadata.name) that doesn't exist can't be used to create a job
	if err := r.ApiServerClient.List(ctx, deploymentList, opts...); err != nil || len(deploymentList.Items) == 0 {
		log.Error(err, "Failed to list Resources")
		return ctrl.Result{}, nil
	}

	job, err := r.CreateJobTemplate(&deploymentList.Items[0], *jobRequest)
	if err != nil {
		log.Error(err, "Failed to create Job Template")
		return ctrl.Result{}, nil
	}

	createJobErr := r.CacheClient.Create(ctx, job)
	if createJobErr != nil {
		log.Error(createJobErr, "Failed to create Job resource")
		return ctrl.Result{}, createJobErr
	}

	return ctrl.Result{}, nil
}

func (r *JobRequestReconciler) CreateJobTemplate(resource *appsv1.Deployment, jobRequest platformv1.JobRequest) (*batch.Job, error) {
	targetContainer := retrieveContainerFromResource(resource, jobRequest)

	if len(targetContainer) == 0 {
		return nil, errors.New("container not found in resource")
	}

	job := batch.Job{}
	jobTemplatePodSpec := *resource.Spec.Template.DeepCopy()
	jobTemplatePodSpec.Spec.Containers = targetContainer
	jobTemplatePodSpec.Spec.RestartPolicy = "Never"
	job.ObjectMeta.Labels = make(map[string]string)
	job.ObjectMeta.Annotations = make(map[string]string)
	job.ObjectMeta.Name = resource.Name
	job.ObjectMeta.Namespace = resource.Namespace
	job.Spec.Template = jobTemplatePodSpec

	maps.Copy(job.ObjectMeta.Annotations, resource.ObjectMeta.Annotations)
	maps.Copy(job.ObjectMeta.Labels, resource.ObjectMeta.Labels)

	if err := ctrl.SetControllerReference(&jobRequest, &job, r.Scheme); err != nil {
		return &job, err
	}

	return &job, nil
}

func retrieveContainerFromResource(resource *appsv1.Deployment, jobRequest platformv1.JobRequest) []v1.Container {

	targetContainer := make([]v1.Container, 0)

	for _, c := range resource.Spec.Template.Spec.Containers {
		if c.Name == jobRequest.Spec.ContainerFrom.ContainerName {
			c.Command = []string{jobRequest.Spec.Command}
			c.Args = jobRequest.Spec.Args
			targetContainer = append(targetContainer, c)
		}
	}

	return targetContainer
}

// SetupWithManager sets up the controller with the Manager.
func (r *JobRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1.JobRequest{}).
		Named("jobrequest").
		Complete(r)
}
