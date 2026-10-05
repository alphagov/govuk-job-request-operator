package controller

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const eventuallyTimeout = 10 * time.Second

func jobRequestBuilder(jobRequestName, resourceName, resourceNamespace, containerName string) *platformv1.JobRequest {
	return &platformv1.JobRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobRequestName,
			Namespace: resourceNamespace,
			Annotations: map[string]string{
				"platform.publishing.service.gov.uk/requested-by": "arn:aws:sts::123456789012:assumed-role/user.name-platformengineer/environment-platformengineer",
			},
		},
		Spec: platformv1.JobRequestSpec{
			ContainerFrom: platformv1.JobRequestContainerFrom{
				PodSpecFrom: platformv1.JobRequestPodSpecFrom{
					Group: "apps/v1",
					Kind:  "Deployment",
					Name:  resourceName,
				},
				ContainerName: containerName,
			},
			Command: "echo",
			Args:    []string{"Hello, World!"},
		},
	}
}

func jobRequestReviewBuilder(jobRequestName, resourceNamespace, jobRequestReviewName, decision string) *platformv1.JobRequestReview {
	return &platformv1.JobRequestReview{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobRequestReviewName,
			Namespace: resourceNamespace,
			Annotations: map[string]string{
				"platform.publishing.service.gov.uk/reviewed-by": "arn:aws:sts::123456789012:assumed-role/other.name-platformengineer/environment-platformengineer",
			},
		},
		Spec: platformv1.JobRequestReviewSpec{
			JobRequestName: jobRequestName,
			Decision:       decision,
			Description:    "A description",
		},
	}
}

func deploymentBuilder(resourceName, resourceNamespace string) *appsv1.Deployment {
	var replicasNum int32 = 1
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: resourceNamespace,
			Annotations: map[string]string{
				"foo": "bar",
			},
			Labels: map[string]string{
				"fizz": "buzz",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicasNum,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "foo",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "foo",
					},
				},
				Spec: v1.PodSpec{
					RestartPolicy: "Always",
					SecurityContext: &v1.PodSecurityContext{
						RunAsUser:    new(int64(1001)),
						RunAsGroup:   new(int64(1001)),
						FSGroup:      new(int64(1001)),
						RunAsNonRoot: new(true),
						SeccompProfile: &v1.SeccompProfile{
							Type: "RuntimeDefault",
						},
					},
					Containers: []v1.Container{
						{
							Name:  "foo",
							Image: "foo/bar",
							Env: []v1.EnvVar{
								{
									Name:  "foo",
									Value: "bar",
								},
							},
							SecurityContext: &v1.SecurityContext{
								AllowPrivilegeEscalation: new(false),
								Capabilities: &v1.Capabilities{
									Drop: []v1.Capability{
										"all",
									},
								},
								ReadOnlyRootFilesystem: new(true),
							},
						},
					},
				},
			},
		},
	}
}

func jobBuilder(jobRequest *platformv1.JobRequest) *batchv1.Job {
	job := &batchv1.Job{}

	job.Namespace = jobRequest.Namespace
	job.Name = jobRequest.Name

	job.Spec = batchv1.JobSpec{
		Template: v1.PodTemplateSpec{
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{Name: "container", Image: "busybox:1"},
				},
				RestartPolicy: "Never",
			},
		},
		Suspend: new(true),
	}

	return job
}

func createDeployment(ctx context.Context, k8sClient client.Client, deploymentName, namespace string) *appsv1.Deployment {
	By("Creating a deployment")
	deployment := deploymentBuilder(deploymentName, namespace)

	Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

	return deployment
}

func createJobRequest(ctx context.Context, k8sClient client.Client, requestName, deploymentName, namespace, containerName string) *platformv1.JobRequest {
	By(fmt.Sprintf("Creating JobRequest %s", requestName))
	jobRequest := jobRequestBuilder(requestName, deploymentName, namespace, containerName)

	Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

	return jobRequest
}

func createJobRequestReview(ctx context.Context, k8sClient client.Client, requestName, namespace, reviewName, decision string) *platformv1.JobRequestReview {
	By(fmt.Sprintf("Creating JobRequestReview %s which reviews JobRequest %s as %s", reviewName, requestName, decision))
	jobRequestReview := jobRequestReviewBuilder(requestName, namespace, reviewName, decision)

	Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

	return jobRequestReview
}

func expectJobRequestToBePending(ctx context.Context, k8sClient client.Client, jobRequest *platformv1.JobRequest) *eventsv1.EventList {
	return expectJobRequestToHaveFinalState(ctx, k8sClient, jobRequest, platformv1.JobRequestPending)
}

func expectJobRequestToBeApproved(ctx context.Context, k8sClient client.Client, jobRequest *platformv1.JobRequest) *eventsv1.EventList {
	return expectJobRequestToHaveFinalState(ctx, k8sClient, jobRequest, platformv1.JobRequestApproved)
}

func expectJobRequestToBeMalformed(ctx context.Context, k8sClient client.Client, jobRequest *platformv1.JobRequest) *eventsv1.EventList {
	return expectJobRequestToHaveFinalState(ctx, k8sClient, jobRequest, platformv1.JobRequestMalformed)
}

func expectJobRequestToHaveStateHistory(ctx context.Context, k8sClient client.Client, jobRequest *platformv1.JobRequest, expectedStates []platformv1.JobRequestState) *eventsv1.EventList {
	By(fmt.Sprintf("Waiting for Job Request to have gone through states %v", expectedStates))
	namespacedName := types.NamespacedName{
		Name:      jobRequest.Name,
		Namespace: jobRequest.Namespace,
	}

	eventList := &eventsv1.EventList{}
	eventOpts := []client.ListOption{
		client.MatchingFields{"reportingController": "jobrequest-controller"},
	}

	finalState := expectedStates[len(expectedStates)-1]

	eventuallyCtx, cancelFunc := context.WithTimeout(ctx, eventuallyTimeout)
	defer cancelFunc()

	Eventually(eventuallyCtx, func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, namespacedName, jobRequest)).To(Succeed())
		g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
		g.Expect(jobRequest.Status.State).To(Equal(finalState))
		g.Expect(eventList.Items).To(HaveLen(len(expectedStates)))
		for i, expectedState := range expectedStates {
			g.Expect(eventList.Items[i].Reason).To(Equal(string(expectedState)))
		}
	}).Should(Succeed())

	return eventList
}

func expectJobRequestToHaveCurrentState(ctx context.Context, k8sClient client.Client, jobRequest *platformv1.JobRequest, currentState platformv1.JobRequestState) {
	By(fmt.Sprintf("Waiting for Job Request to have state %s", currentState))
	namespacedName := types.NamespacedName{
		Name:      jobRequest.Name,
		Namespace: jobRequest.Namespace,
	}

	eventuallyCtx, cancelFunc := context.WithTimeout(ctx, eventuallyTimeout)
	defer cancelFunc()

	Eventually(eventuallyCtx, func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, namespacedName, jobRequest)).To(Succeed())
		g.Expect(jobRequest.Status.State).To(Equal(currentState))
	}).Should(Succeed())
}

func expectJobRequestToHaveFinalState(ctx context.Context, k8sClient client.Client, jobRequest *platformv1.JobRequest, finalState platformv1.JobRequestState) *eventsv1.EventList {
	By(fmt.Sprintf("Waiting for Job Request to have reached state %s", finalState))
	namespacedName := types.NamespacedName{
		Name:      jobRequest.Name,
		Namespace: jobRequest.Namespace,
	}

	eventList := &eventsv1.EventList{}
	eventOpts := []client.ListOption{
		client.MatchingFields{"reportingController": "jobrequest-controller"},
	}

	eventuallyCtx, cancelFunc := context.WithTimeout(ctx, eventuallyTimeout)
	defer cancelFunc()

	Eventually(eventuallyCtx, func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, namespacedName, jobRequest)).To(Succeed())
		g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
		g.Expect(jobRequest.Status.State).To(Equal(finalState))

		numberOfEvents := len(eventList.Items)
		finalEvent := eventList.Items[numberOfEvents-1]
		g.Expect(finalEvent.Reason).To(Equal(string(finalState)))
	}).Should(Succeed())

	return eventList
}

func updateJobRequestStatus(ctx context.Context, k8sClient client.Client, jobRequest *platformv1.JobRequest, status platformv1.JobRequestStatus) {
	By(fmt.Sprintf("Updating JobRequest %s to state %s with review %s", jobRequest.Name, status.State, status.ReviewName))

	jobRequest.Status = status
	Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())
}
