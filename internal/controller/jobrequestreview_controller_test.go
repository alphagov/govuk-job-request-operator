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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
)

var _ = Describe("JobRequestReview Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		ctx, cancel := context.WithCancel(context.Background())
		SetDefaultEventuallyTimeout(10 * time.Second)

		reviewNamespaceName := "apps-review"
		jobRequestName := "request"
		jobRequestReviewName := "review"
		deploymentName := "deployment"
		containerName := "foo"
		eventOpts := []client.ListOption{
			client.MatchingFields{"reportingController": "jobrequestreview-controller"},
		}

		jobRequestNamespaceName := types.NamespacedName{
			Name:      jobRequestName,
			Namespace: reviewNamespaceName,
		}

		jobRequestReviewNamespaceName := types.NamespacedName{
			Name:      jobRequestReviewName,
			Namespace: reviewNamespaceName,
		}

		appsNamespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      reviewNamespaceName,
				Namespace: reviewNamespaceName,
			},
		}

		BeforeAll(func() {
			By("create the manager")
			mgr, err := ctrl.NewManager(cfg, ctrl.Options{
				Scheme: scheme.Scheme,
			})
			Expect(err).ToNot(HaveOccurred())

			By("create the JobRequestReview controller")
			err = (&JobRequestReviewReconciler{
				CacheClient:     mgr.GetClient(),
				ApiServerClient: mgr.GetAPIReader(),
				Scheme:          mgr.GetScheme(),
				Recorder:        mgr.GetEventRecorder("jobrequestreview-controller"),
			}).SetupControllerWithManager(mgr)

			go func() {
				defer GinkgoRecover()
				err = mgr.Start(ctx)
				Expect(err).ToNot(HaveOccurred(), "failed to run manager")
			}()

			By("create apps namespace")
			err = k8sClient.Create(ctx, appsNamespace)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterAll(func() {
			By("delete apps namespace")
			err := k8sClient.Delete(ctx, appsNamespace)
			Expect(err).NotTo(HaveOccurred())
			By("stop the manager")
			cancel()
		})

		AfterEach(func() {
			By("tear down test resources and removing JobReview resource")
			var background metav1.DeletionPropagation = "Background"
			var graceSecs int64 = 0
			opts := &client.DeleteAllOfOptions{}
			opts.Namespace = reviewNamespaceName
			opts.GracePeriodSeconds = &graceSecs
			opts.PropagationPolicy = &background
			By("tearing down the JobRequests")
			Expect(k8sClient.DeleteAllOf(ctx, &platformv1.JobRequest{}, opts)).To(Succeed())

			By("tearing down the JobRequestReviews")
			Expect(k8sClient.DeleteAllOf(ctx, &platformv1.JobRequestReview{}, opts)).To(Succeed())

			By("tearing down the Deployments")
			Expect(k8sClient.DeleteAllOf(ctx, &appsv1.Deployment{}, opts)).To(Succeed())

			By("tearing down the Events")
			Expect(k8sClient.DeleteAllOf(ctx, &eventsv1.Event{}, opts)).To(Succeed())
		})

		It("should successfully reconcile with JobRequestReview state as JobRequestNotFound if the corresponding JobRequest doesn't exist", func() {
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, "Approved")

			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequestReview.Status.State).To(Equal(platformv1.JobRequestReviewNotFound))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestReviewNotFound)))
			}).Should(Succeed())
		})

		It("should successfully reconcile if the corresponding JobRequest status is initally empty", func() {
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, "Approved")
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, reviewNamespaceName, containerName)

			jobRequestStatus := platformv1.JobRequestStatus{
				JobName:    deploymentName,
				State:      platformv1.JobRequestPending,
				ReviewName: jobRequestReviewName,
			}

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequestReview.Status.State).To(Equal(platformv1.JobRequestReviewState("")))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Pending"))
			}).Should(Succeed())

			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequestReview.Status.State).To(Equal(platformv1.JobRequestReviewApproved))
				g.Expect(eventList.Items).To(HaveLen(2))
				g.Expect(eventList.Items[1].Reason).To(Equal(string(platformv1.JobRequestReviewApproved)))
			}, 20*time.Second).Should(Succeed())
		})

		It("should successfully reconcile with JobRequestReview state as JobRequestMalformed if the corresponding JobRequest is Malformed", func() {
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, "Approved")
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, reviewNamespaceName, containerName)

			jobRequestStatus := platformv1.JobRequestStatus{
				JobName:    deploymentName,
				State:      platformv1.JobRequestMalformed,
				ReviewName: jobRequestReviewName,
			}

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequestReview.Status.State).To(Equal(platformv1.JobRequestReviewMalformed))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestReviewMalformed)))
			}).Should(Succeed())
		})

		It("should successfully reconcile when JobRequestReview is Approved", func() {
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, "Approved")
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, reviewNamespaceName, containerName)

			jobRequestStatus := platformv1.JobRequestStatus{
				State: platformv1.JobRequestPending,
			}

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequestReview.Status.State).To(Equal(platformv1.JobRequestReviewApproved))
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestApproved))
				g.Expect(jobRequest.Status.ReviewName).To(Equal(jobRequestReviewName))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestReviewApproved)))
			}).Should(Succeed())
		})

		It("should successfully reconcile when JobRequestReview is Rejected", func() {
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, "Rejected")
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, reviewNamespaceName, containerName)

			jobRequestStatus := platformv1.JobRequestStatus{
				State: platformv1.JobRequestPending,
			}

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequestReview.Status.State).To(Equal(platformv1.JobRequestReviewRejected))
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestRejected))
				g.Expect(jobRequest.Status.ReviewName).To(Equal(jobRequestReviewName))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestReviewRejected)))
			}).Should(Succeed())
		})
	})
})
