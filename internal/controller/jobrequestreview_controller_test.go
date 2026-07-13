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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
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

		jobRequestNamespaceName := types.NamespacedName{
			Name:      jobRequestName,
			Namespace: reviewNamespaceName,
		}

		jobRequestReviewNamespaceName := types.NamespacedName{
			Name:      jobRequestReviewName,
			Namespace: reviewNamespaceName,
		}

		appsNamespace := &v1.Namespace{
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
			Expect(k8sClient.DeleteAllOf(ctx, &platformv1.JobRequest{}, opts)).To(Succeed())
			Expect(k8sClient.DeleteAllOf(ctx, &platformv1.JobRequestReview{}, opts)).To(Succeed())
			Expect(k8sClient.DeleteAllOf(ctx, &appsv1.Deployment{}, opts)).To(Succeed())
			Expect(k8sClient.DeleteAllOf(ctx, &batch.Job{}, opts)).To(Succeed())
		})

		It("should successfully reconcile with JobRequestReview state as JobRequestNotFound if the corresponding JobRequest doesn't exist", func() {
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, "Approved")

			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
				g.Expect(jobRequestReview.Status.State).To(Equal("JobRequestNotFound"))
			}).Should(Succeed())
		})

		It("should successfully reconcile if the corresponding JobRequest status is initally empty", func() {
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, "Approved")
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, reviewNamespaceName, containerName)

			jobRequestStatus := platformv1.JobRequestStatus{
				JobName:    deploymentName,
				State:      "Pending",
				ReviewName: jobRequestReviewName,
			}

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
				g.Expect(jobRequestReview.Status.State).To(Equal(""))
			}).Should(Succeed())

			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
				g.Expect(jobRequestReview.Status.State).To(Equal("Approved"))
			}, 20*time.Second).Should(Succeed())
		})

		It("should successfully reconcile with JobRequestReview state as JobRequestMalformed if the corresponding JobRequest is Malformed", func() {
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, "Approved")
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, reviewNamespaceName, containerName)

			jobRequestStatus := platformv1.JobRequestStatus{
				JobName:    deploymentName,
				State:      "Malformed",
				ReviewName: jobRequestReviewName,
			}

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
				g.Expect(jobRequestReview.Status.State).To(Equal("JobRequestMalformed"))
			}).Should(Succeed())
		})

		It("should successfully reconcile when JobRequestReview is Approved", func() {
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, "Approved")
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, reviewNamespaceName, containerName)

			jobRequestStatus := platformv1.JobRequestStatus{
				State: "Pending",
			}

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
				g.Expect(jobRequestReview.Status.State).To(Equal("Approved"))
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal("Approved"))
				g.Expect(jobRequest.Status.ReviewName).To(Equal(jobRequestReviewName))
			}).Should(Succeed())
		})

		It("should successfully reconcile when JobRequestReview is Rejected", func() {
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, "Rejected")
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, reviewNamespaceName, containerName)

			jobRequestStatus := platformv1.JobRequestStatus{
				State: "Pending",
			}

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
				g.Expect(jobRequestReview.Status.State).To(Equal("Rejected"))
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal("Rejected"))
				g.Expect(jobRequest.Status.ReviewName).To(Equal(jobRequestReviewName))
			}).Should(Succeed())
		})
	})
})
