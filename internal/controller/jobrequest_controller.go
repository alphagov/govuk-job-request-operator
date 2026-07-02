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

	"github.com/go-logr/logr"
)

type JobRequestReconciler struct {
	CacheClient     client.Client
	ApiServerClient client.Reader
	Scheme          *runtime.Scheme
	Log             logr.Logger
}

// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequests/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;create

func (r *JobRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	jobRequest := &platformv1.JobRequest{}

	err := r.getJobRequest(ctx, req.NamespacedName, jobRequest)
	if err != nil {
		return ctrl.Result{}, nil
	}

	// TODO: consider bailing out early based on the state eg. Failed (no need to lookup deployments and review etc.)

	resourceResult, resourceList := r.getTargetResource(ctx, jobRequest)
	if resourceResult != nil {
		return *resourceResult, nil
	}

	jobTemplate, err := r.createJobTemplate(ctx, &resourceList.Items[0], *jobRequest)
	if err != nil {
		return ctrl.Result{}, nil
	}

	jobRequestState := r.calculateState(ctx, jobRequest)

	return r.handleState(ctx, jobRequestState, jobRequest, jobTemplate)
}

func (r *JobRequestReconciler) createJobTemplate(ctx context.Context, resource *appsv1.Deployment, jobRequest platformv1.JobRequest) (*batch.Job, error) {
	targetContainer := retrieveContainerFromResource(resource, jobRequest)

	if len(targetContainer) == 0 {
		err := errors.New("container not found in resource")
		r.Log.Error(err, "Target container, to create the job from is not found in target resource")
		r.setState(ctx, &jobRequest, "Failed")
		return nil, err
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
		r.Log.Error(err, "Failed to create Job Template")
		r.setState(ctx, &jobRequest, "Failed")
		return &job, err
	}

	return &job, nil
}

func (r *JobRequestReconciler) getJobRequest(ctx context.Context, namespaceName client.ObjectKey, jobRequest *platformv1.JobRequest) error {
	err := r.CacheClient.Get(ctx, namespaceName, jobRequest)
	if apierrors.IsNotFound(err) {
		r.Log.Error(err, "JobRequest resource not found. This is usually because the resource was deleted or not created. Ignoring and ending reconciliation")
		r.setState(ctx, jobRequest, "Failed")
		return err
	}

	if err != nil {
		r.Log.Error(err, "Failed to get JobRequest. Suspect the jobrequest is malformed")
		r.setState(ctx, jobRequest, "Malformed")
		return err
	}

	return nil
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

func (r *JobRequestReconciler) calculateState(ctx context.Context, jobRequest *platformv1.JobRequest) string {
	jobRequestReviewList := &platformv1.JobRequestReviewList{}
	opts := []client.ListOption{
		client.MatchingFields{"spec.jobRequestName": jobRequest.GetObjectMeta().GetName()},
	}

	if err := r.ApiServerClient.List(ctx, jobRequestReviewList, opts...); err != nil {
		r.Log.Error(err, "Failed to list JobRequestReview")
		return "Pending"
	}

	if len(jobRequestReviewList.Items) == 0 {
		r.Log.Info("No JobRequestReview has been found")
		return "Pending"
	}

	return jobRequest.Status.State
}

func (r *JobRequestReconciler) handleState(ctx context.Context, jobRequestState string, jobRequest *platformv1.JobRequest, jobTemplate client.Object) (ctrl.Result, error) {
	switch jobRequestState {
	case "Pending":
		r.setState(ctx, jobRequest, "Pending")
		return ctrl.Result{}, nil
	case "Approved":
		err := r.CacheClient.Create(ctx, jobTemplate)
		if err != nil {
			r.Log.Error(err, "Failed to create Job resource")
			r.setState(ctx, jobRequest, "Failed")
			return ctrl.Result{}, err
		}
		r.setState(ctx, jobRequest, "Started")
		return ctrl.Result{}, nil
	case "Failed":
		r.setState(ctx, jobRequest, "Failed")
		return ctrl.Result{}, nil
	case "Rejected", "Started", "Completed", "Malformed":
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, nil
	}
}

func (r *JobRequestReconciler) setState(ctx context.Context, jobRequest *platformv1.JobRequest, state string) {
	jobRequest.Status.State = state
	err := r.CacheClient.Status().Update(ctx, jobRequest)
	if err != nil {
		r.Log.Error(err, "Failed to UPDATE JobRequest resource", "errored_obj", jobRequest)
	}
}

func (r *JobRequestReconciler) getTargetResource(ctx context.Context, jobRequest *platformv1.JobRequest) (*ctrl.Result, *appsv1.DeploymentList) {
	deploymentList := &appsv1.DeploymentList{}
	opts := []client.ListOption{
		client.MatchingFields{"metadata.name": jobRequest.Spec.ContainerFrom.PodSpecFrom.Name},
	}

	if err := r.ApiServerClient.List(ctx, deploymentList, opts...); err != nil || len(deploymentList.Items) == 0 {
		r.Log.Error(err, "Failed to list Resources")
		r.setState(ctx, jobRequest, "Failed")
		return &ctrl.Result{}, nil
	}

	return nil, deploymentList
}

func (r *JobRequestReconciler) SetupControllerWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1.JobRequest{}).
		Named("jobrequest").
		Complete(r)
}
