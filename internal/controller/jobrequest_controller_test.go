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

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
)

var _ = Describe("JobRequest Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		ctx, cancel := context.WithCancel(context.Background())
		SetDefaultEventuallyTimeout(10 * time.Second)

		appNamespaceName := "apps"
		deploymentName := "deployment"
		containerName := "foo"
		jobRequestName := "request"
		jobRequestReviewName := "review"
		jobOpts := []client.ListOption{
			client.MatchingFields{"metadata.name": jobRequestName},
		}
		eventOpts := []client.ListOption{
			client.MatchingFields{"reportingController": "jobrequest-controller"},
		}

		jobRequestNamespaceName := types.NamespacedName{
			Name:      jobRequestName,
			Namespace: appNamespaceName,
		}

		appsNamespace := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      appNamespaceName,
				Namespace: appNamespaceName,
			},
		}

		BeforeAll(func() {
			By("create the manager")
			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme: scheme.Scheme,
			})
			Expect(err).ToNot(HaveOccurred())

			By("create the JobRequest controller")
			err = (&JobRequestReconciler{
				CacheClient:     mgr.GetClient(),
				ApiServerClient: mgr.GetAPIReader(),
				Scheme:          mgr.GetScheme(),
				Recorder:        mgr.GetEventRecorder("jobrequest-controller"),
			}).SetupControllerWithManager(mgr)

			go func() {
				defer GinkgoRecover()
				err = mgr.Start(ctx)
				Expect(err).ToNot(HaveOccurred(), "failed to run manager")
			}()

			By("create apps namespace")
			Expect(k8sClient.Create(ctx, appsNamespace)).To(Succeed())
		})

		AfterAll(func() {
			By("delete apps namespace")
			Expect(k8sClient.Delete(ctx, appsNamespace)).To(Succeed())
			By("stop the manager")
			cancel()
		})

		AfterEach(func() {
			var background metav1.DeletionPropagation = "Background"
			var graceSecs int64 = 0
			opts := &client.DeleteAllOfOptions{}
			opts.Namespace = appNamespaceName
			opts.GracePeriodSeconds = &graceSecs
			opts.PropagationPolicy = &background

			By("tearing down the JobRequests")
			Expect(k8sClient.DeleteAllOf(ctx, &platformv1.JobRequest{}, opts)).To(Succeed())

			By("tearing down the JobRequestReviews")
			Expect(k8sClient.DeleteAllOf(ctx, &platformv1.JobRequestReview{}, opts)).To(Succeed())

			By("tearing down the Deployments")
			Expect(k8sClient.DeleteAllOf(ctx, &appsv1.Deployment{}, opts)).To(Succeed())

			By("tearing down the Jobs")
			Expect(k8sClient.DeleteAllOf(ctx, &batch.Job{}, opts)).To(Succeed())

			By("tearing down the Events")
			Expect(k8sClient.DeleteAllOf(ctx, &eventsv1.Event{}, opts)).To(Succeed())
		})

		It("should successfully reconcile when JobRequest is 'Approved' and the job created", func() {
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, appNamespaceName, containerName)
			targetResource := deploymentBuilder(deploymentName, appNamespaceName)
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, appNamespaceName, jobRequestReviewName, "Approved")

			jobRequestStatus := platformv1.JobRequestStatus{
				State:      "Approved",
				ReviewName: jobRequestReviewName,
			}

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal("Pending"))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Pending"))
			}).Should(Succeed())

			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal("Started"))
				g.Expect(jobRequest.Status.JobName).To(Equal(jobRequestName))
				g.Expect(eventList.Items).To(HaveLen(3))
				g.Expect(eventList.Items[1].Reason).To(Equal("Approved"))
				g.Expect(eventList.Items[2].Reason).To(Equal("Started"))
			}).Should(Succeed())

			jobList := &batch.JobList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, jobList, jobOpts...)).To(Succeed())
				g.Expect(jobList.Items).To(HaveLen(1))
				g.Expect(jobList.Items[0].GetName()).To(Equal(jobRequestName))
				g.Expect(jobList.Items[0].GetNamespace()).To(Equal(appNamespaceName))
				g.Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].Name).To(Equal("foo"))
				g.Expect(jobList.Items[0].Spec.Template.Spec.Containers).To(HaveLen(1))
				g.Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].Image).To(Equal("foo/bar"))
				g.Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].Env[0].Name).To(Equal("foo"))
				g.Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].Env[0].Value).To(Equal("bar"))
				g.Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation).To(Equal(ptr.To(false)))
				g.Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].SecurityContext.Capabilities.Drop[0]).To(BeEquivalentTo("all"))
				g.Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem).To(Equal(ptr.To(true)))
				g.Expect(jobList.Items[0].Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(Equal(ptr.To(true)))
				g.Expect(jobList.Items[0].Spec.Template.Spec.SecurityContext.RunAsUser).To(Equal(ptr.To(int64(1001))))
				g.Expect(jobList.Items[0].Spec.Template.Spec.SecurityContext.RunAsGroup).To(Equal(ptr.To(int64(1001))))
				g.Expect(jobList.Items[0].Spec.Template.Spec.SecurityContext.FSGroup).To(Equal(ptr.To(int64(1001))))
				g.Expect(jobList.Items[0].Spec.Template.Spec.SecurityContext.SeccompProfile.Type).To(BeEquivalentTo("RuntimeDefault"))
				g.Expect(jobList.Items[0].Spec.Template.Spec.RestartPolicy).To(Equal(v1.RestartPolicyNever))
				g.Expect(jobList.Items[0].Annotations["foo"]).To(Equal("bar"))
				g.Expect(jobList.Items[0].Labels["fizz"]).To(Equal("buzz"))
				g.Expect(jobList.Items[0].ObjectMeta.GetOwnerReferences()).NotTo(BeNil())
				g.Expect(jobList.Items[0].ObjectMeta.GetOwnerReferences()[0].Name).To(Equal(jobRequestName))
				g.Expect(jobList.Items[0].ObjectMeta.GetOwnerReferences()[0].Kind).To(Equal("JobRequest"))
			}).Should(Succeed())
		})

		It("should successfully reconcile if we cannot retrieve the target resource in the JobRequest from the cluster and the job should not be created", func() {
			jobRequest := jobRequestBuilder(jobRequestName, "example-app", appNamespaceName, "example-container")
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal("Malformed"))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Malformed"))
			}).Should(Succeed())
		})

		It("should successfully reconcile if the the target container doesn't exist and the job should not be created", func() {
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, appNamespaceName, "non-existent-container")
			targetResource := deploymentBuilder(deploymentName, appNamespaceName)

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal("Malformed"))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Malformed"))
			}).Should(Succeed())
		})

		It("should successfully reconcile when JobRequest is 'Pending' and the job should not be created", func() {
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, appNamespaceName, containerName)
			targetResource := deploymentBuilder(deploymentName, appNamespaceName)

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			jobList := &batch.JobList{}
			Expect(k8sClient.List(ctx, jobList, jobOpts...)).To(Succeed())
			Expect(jobList.Items).To(BeEmpty())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal("Pending"))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Pending"))
			}).Should(Succeed())
		})

		It("should successfully reconcile when JobRequest is 'Rejected' and the job should not be created", func() {
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, appNamespaceName, containerName)
			targetResource := deploymentBuilder(deploymentName, appNamespaceName)
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, appNamespaceName, jobRequestReviewName, "Rejected")

			jobRequestStatus := platformv1.JobRequestStatus{
				State:      "Rejected",
				ReviewName: "test",
			}

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal("Pending"))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Pending"))
			}).Should(Succeed())

			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

			jobList := &batch.JobList{}
			Expect(k8sClient.List(ctx, jobList, jobOpts...)).To(Succeed())
			Expect(jobList.Items).To(BeEmpty())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal("Rejected"))
				g.Expect(eventList.Items).To(HaveLen(2))
				g.Expect(eventList.Items[1].Reason).To(Equal("Rejected"))
			}).Should(Succeed())
		})

		It("should successfully reconcile with no job created when JobRequest is 'Malformed'", func() {
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, appNamespaceName, containerName)
			targetResource := deploymentBuilder(jobRequestName, appNamespaceName)
			jobRequestReview := jobRequestReviewBuilder(deploymentName, appNamespaceName, jobRequestReviewName, "Approved")

			jobRequestStatus := platformv1.JobRequestStatus{
				State: "Malformed",
			}

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			jobList := &batch.JobList{}

			Expect(k8sClient.List(ctx, jobList, jobOpts...)).To(Succeed())
			Expect(jobList.Items).To(BeEmpty())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal("Malformed"))
			}).Should(Succeed())
		})
	})
})
