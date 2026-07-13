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
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/client-go/tools/events"

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
	Recorder        events.EventRecorder
	Log             logr.Logger
}

const pending = "Pending"

// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequests/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.publishing.service.gov.uk,resources=jobrequests/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;create
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

func (r *JobRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	jobRequest := &platformv1.JobRequest{}

	found := r.getJobRequest(ctx, req.NamespacedName, jobRequest)
	if !found {
		return ctrl.Result{}, nil
	}

	if endReconcile(jobRequest.Status.State) {
		return ctrl.Result{}, nil
	}

	resourceResult, resourceList := r.getTargetResource(ctx, jobRequest)
	if resourceResult != nil {
		return *resourceResult, nil
	}

	jobTemplate := r.createJobTemplate(ctx, &resourceList.Items[0], *jobRequest)
	if jobTemplate == nil {
		return ctrl.Result{}, nil
	}

	jobRequestState := r.calculateState(ctx, jobRequest)

	return r.handleState(ctx, jobRequestState, jobRequest, jobTemplate)
}

func (r *JobRequestReconciler) getJobRequest(ctx context.Context, namespaceName client.ObjectKey, jobRequest *platformv1.JobRequest) bool {
	err := r.CacheClient.Get(ctx, namespaceName, jobRequest)
	if err != nil {
		var errorLogMessage string
		if apierrors.IsNotFound(err) {
			errorLogMessage = "JobRequest resource not found. This is usually because the resource was deleted or not created. Ignoring and ending reconciliation"
		} else {
			errorLogMessage = "Failed to deserialize JobRequest. Ignoring and ending reconciliation"
		}

		r.Log.Error(err, errorLogMessage)
		return false
	}
	return true
}

func endReconcile(jobRequestState string) bool {
	return slices.Contains([]string{
		"Completed",
		"Failed",
		"Malformed",
	}, jobRequestState)
}

func (r *JobRequestReconciler) getTargetResource(ctx context.Context, jobRequest *platformv1.JobRequest) (*ctrl.Result, *appsv1.DeploymentList) {
	deploymentList := &appsv1.DeploymentList{}
	opts := []client.ListOption{
		client.MatchingFields{"metadata.name": jobRequest.Spec.ContainerFrom.PodSpecFrom.Name},
	}

	if err := r.ApiServerClient.List(ctx, deploymentList, opts...); err != nil || len(deploymentList.Items) == 0 {
		r.Log.Error(err, "Failed to retrieve target resource")
		r.Recorder.Eventf(jobRequest, nil, corev1.EventTypeWarning, "Failed", "None", "Target resource could not be found")
		r.setState(ctx, jobRequest, "Failed")
		return &ctrl.Result{}, nil
	}

	return nil, deploymentList
}

func (r *JobRequestReconciler) createJobTemplate(ctx context.Context, resource *appsv1.Deployment, jobRequest platformv1.JobRequest) *batch.Job {
	targetContainer := retrieveContainerFromResource(resource, jobRequest)

	if len(targetContainer) == 0 {
		err := errors.New("container not found in resource")
		r.Log.Error(err, "Target container, to create the job from is not found in target resource")
		r.Recorder.Eventf(&jobRequest, nil, corev1.EventTypeWarning, "Failed", "None", "Target container on Deployment could not be found")
		r.setState(ctx, &jobRequest, "Failed")

		return nil
	}

	job := batch.Job{}
	job.Labels = make(map[string]string)
	job.Annotations = make(map[string]string)
	job.Name = resource.Name
	job.Namespace = resource.Namespace
	jobTemplatePodSpec := *resource.Spec.Template.DeepCopy()
	jobTemplatePodSpec.Spec.Containers = targetContainer
	jobTemplatePodSpec.Spec.RestartPolicy = "Never"
	job.Spec.Template = jobTemplatePodSpec

	maps.Copy(job.Annotations, resource.Annotations)
	maps.Copy(job.Labels, resource.Labels)

	if err := ctrl.SetControllerReference(&jobRequest, &job, r.Scheme); err != nil {
		r.Log.Error(err, "Failed to create Job Template from Deployment")
		r.Recorder.Eventf(&jobRequest, nil, corev1.EventTypeWarning, "Failed", "None", "JobTemplate could not be created from Deployment")
		r.setState(ctx, &jobRequest, "Failed")
		return &job
	}

	return &job
}

func retrieveContainerFromResource(resource *appsv1.Deployment, jobRequest platformv1.JobRequest) []corev1.Container {
	targetContainer := make([]corev1.Container, 0)

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
		r.Log.Error(err, "Failed to retrieve JobRequestReview")
		return pending
	}

	if len(jobRequestReviewList.Items) == 0 {
		if jobRequest.Status.State == "" {
			r.Recorder.Eventf(jobRequest, nil, corev1.EventTypeNormal, "Pending", "None", "JobRequest is waiting for a JobRequestReview")
			return pending
		}
		return pending
	}

	return jobRequest.Status.State
}

func (r *JobRequestReconciler) handleState(ctx context.Context, jobRequestState string, jobRequest *platformv1.JobRequest, jobTemplate client.Object) (ctrl.Result, error) {
	switch jobRequestState {
	case pending:
		r.setState(ctx, jobRequest, pending)
		return ctrl.Result{}, nil
	case "Approved":
		r.Recorder.Eventf(jobRequest, nil, corev1.EventTypeNormal, "Approved", "None", "JobRequest is Approved")
		err := r.CacheClient.Create(ctx, jobTemplate)
		if err != nil {
			r.Log.Error(err, "Failed to create Job resource")
			r.Recorder.Eventf(jobRequest, nil, corev1.EventTypeNormal, "Failed", "None", "Failed to create Job")
			r.setState(ctx, jobRequest, "Failed")
			return ctrl.Result{}, err
		}
		jobRequest.Status.JobName = jobTemplate.GetName()
		r.setState(ctx, jobRequest, "Started")
		r.Recorder.Eventf(jobRequest, nil, corev1.EventTypeNormal, "Started", "None", "Job is created")
		return ctrl.Result{}, nil
	case "Rejected":
		r.Recorder.Eventf(jobRequest, nil, corev1.EventTypeNormal, "Rejected", "None", "JobRequest is Rejected")
		return ctrl.Result{}, nil
	case "Failed":
		r.setState(ctx, jobRequest, "Failed")
		return ctrl.Result{}, nil
	case "Started", "Completed", "Malformed":
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, nil
	}
}

func (r *JobRequestReconciler) setState(ctx context.Context, jobRequest *platformv1.JobRequest, state string) {
	jobRequest.Status.State = state
	err := r.CacheClient.Status().Update(ctx, jobRequest)
	if err != nil {
		r.Log.Error(err, "Failed to update status of JobRequest")
	}
}

func (r *JobRequestReconciler) SetupControllerWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1.JobRequest{}).
		Named("jobrequest").
		Complete(r)
}
