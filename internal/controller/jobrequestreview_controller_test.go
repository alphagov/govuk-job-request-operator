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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
)

var _ = Describe("JobRequestReview Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"
		reviewNamespaceName := "apps-review"

		ctx := context.Background()

		appsTypeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: reviewNamespaceName,
		}

		appsNamespace := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      reviewNamespaceName,
				Namespace: reviewNamespaceName,
			},
		}

		BeforeAll(func() {
			By("create apps namespace")
			err := k8sClient.Create(ctx, appsNamespace)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterAll(func() {
			By("delete apps namespace")
			err := k8sClient.Delete(ctx, appsNamespace)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			By("tearing down the test resources")
			By("cleaning up the JobReview")
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

		It("should successfully reconcile if JobRequestReview doesn't exist", func() {
			controllerReconciler := &JobRequestReviewReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: appsTypeNamespacedName,
			})

			Expect(err).NotTo(HaveOccurred())
		})

		It("should successfully reconcile with JobRequestReview state as JobRequestNotFound if the corresponding JobRequest doesn't exist", func() {
			jobRequestReview := &platformv1.JobRequestReview{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: reviewNamespaceName,
					Annotations: map[string]string{
						"platform.publishing.service.gov.uk/reviewed-by": "arn:aws:sts::123456789:assumed-role/user.name-platformengineer/environment-platformengineer",
					},
				},
				Spec: platformv1.JobRequestReviewSpec{
					JobRequestName: resourceName,
					Decision:       "Approved",
					Description:    "A description",
				},
			}

			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			controllerReconciler := &JobRequestReviewReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: appsTypeNamespacedName,
			})

			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, appsTypeNamespacedName, jobRequestReview)).To(Succeed())
			Expect(jobRequestReview.Status.State).To(Equal("JobRequestNotFound"))
		})

		It("should successfully reconcile if the corresponding JobRequest status is empty", func() {
			jobRequestReview := &platformv1.JobRequestReview{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: reviewNamespaceName,
					Annotations: map[string]string{
						"platform.publishing.service.gov.uk/reviewed-by": "arn:aws:sts::123456789:assumed-role/user.name-platformengineer/environment-platformengineer",
					},
				},
				Spec: platformv1.JobRequestReviewSpec{
					JobRequestName: resourceName,
					Decision:       "Approved",
					Description:    "A description",
				},
			}

			jobRequest := &platformv1.JobRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: reviewNamespaceName,
					Annotations: map[string]string{
						"platform.publishing.service.gov.uk/reviewed-by": "arn:aws:sts::123456789:assumed-role/user.name-platformengineer/environment-platformengineer",
					},
				},
				Spec: platformv1.JobRequestSpec{
					ContainerFrom: platformv1.JobRequestContainerFrom{
						PodSpecFrom: platformv1.JobRequestPodSpecFrom{
							Group: "apps/v1",
							Kind:  "Deployment",
							Name:  resourceName,
						},
						ContainerName: "foo-container",
					},
					Command: "echo",
					Args:    []string{"Hello, World!"},
				},
			}

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			controllerReconciler := &JobRequestReviewReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: appsTypeNamespacedName,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(time.Minute))
		})

		It("should successfully reconcile with JobRequestReview state as JobRequestMalformed if the corresponding JobRequest is Malformed", func() {
			jobRequestReview := &platformv1.JobRequestReview{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: reviewNamespaceName,
					Annotations: map[string]string{
						"platform.publishing.service.gov.uk/reviewed-by": "arn:aws:sts::123456789:assumed-role/user.name-platformengineer/environment-platformengineer",
					},
				},
				Spec: platformv1.JobRequestReviewSpec{
					JobRequestName: resourceName,
					Decision:       "Approved",
					Description:    "A description",
				},
			}

			jobRequestStatus := platformv1.JobRequestStatus{
				JobName:    resourceName,
				State:      "Malformed",
				ReviewName: resourceName,
			}

			jobRequest := &platformv1.JobRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: reviewNamespaceName,
					Annotations: map[string]string{
						"platform.publishing.service.gov.uk/reviewed-by": "arn:aws:sts::123456789:assumed-role/user.name-platformengineer/environment-platformengineer",
					},
				},
				Spec: platformv1.JobRequestSpec{
					ContainerFrom: platformv1.JobRequestContainerFrom{
						PodSpecFrom: platformv1.JobRequestPodSpecFrom{
							Group: "apps/v1",
							Kind:  "Deployment",
							Name:  resourceName,
						},
						ContainerName: "foo-container",
					},
					Command: "echo",
					Args:    []string{"Hello, World!"},
				},
			}

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			controllerReconciler := &JobRequestReviewReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: appsTypeNamespacedName,
			})

			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, appsTypeNamespacedName, jobRequestReview)).To(Succeed())
			Expect(jobRequestReview.Status.State).To(Equal("JobRequestMalformed"))
		})

		It("should successfully reconcile when JobRequestReview is Approved", func() {
			jobRequestReview := &platformv1.JobRequestReview{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: reviewNamespaceName,
					Annotations: map[string]string{
						"platform.publishing.service.gov.uk/reviewed-by": "arn:aws:sts::123456789:assumed-role/user.name-platformengineer/environment-platformengineer",
					},
				},
				Spec: platformv1.JobRequestReviewSpec{
					JobRequestName: resourceName,
					Decision:       "Approved",
					Description:    "A description",
				},
			}

			jobRequestStatus := platformv1.JobRequestStatus{
				State: "Pending",
			}

			jobRequest := &platformv1.JobRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: reviewNamespaceName,
					Annotations: map[string]string{
						"platform.publishing.service.gov.uk/reviewed-by": "arn:aws:sts::123456789:assumed-role/user.name-platformengineer/environment-platformengineer",
					},
				},
				Spec: platformv1.JobRequestSpec{
					ContainerFrom: platformv1.JobRequestContainerFrom{
						PodSpecFrom: platformv1.JobRequestPodSpecFrom{
							Group: "apps/v1",
							Kind:  "Deployment",
							Name:  resourceName,
						},
						ContainerName: "foo-container",
					},
					Command: "echo",
					Args:    []string{"Hello, World!"},
				},
			}

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			controllerReconciler := &JobRequestReviewReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: appsTypeNamespacedName,
			})

			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, appsTypeNamespacedName, jobRequestReview)).To(Succeed())
			Expect(jobRequestReview.Status.State).To(Equal("Approved"))

			Expect(k8sClient.Get(ctx, appsTypeNamespacedName, jobRequest)).To(Succeed())
			Expect(jobRequest.Status.State).To(Equal("Approved"))
			Expect(jobRequest.Status.ReviewName).To(Equal("test-resource"))
		})

		It("should successfully reconcile when JobRequestReview is Rejected", func() {
			jobRequestReview := &platformv1.JobRequestReview{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: reviewNamespaceName,
					Annotations: map[string]string{
						"platform.publishing.service.gov.uk/reviewed-by": "arn:aws:sts::123456789:assumed-role/user.name-platformengineer/environment-platformengineer",
					},
				},
				Spec: platformv1.JobRequestReviewSpec{
					JobRequestName: resourceName,
					Decision:       "Rejected",
					Description:    "A description",
				},
			}

			jobRequestStatus := platformv1.JobRequestStatus{
				State: "Pending",
			}

			jobRequest := &platformv1.JobRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: reviewNamespaceName,
					Annotations: map[string]string{
						"platform.publishing.service.gov.uk/reviewed-by": "arn:aws:sts::123456789:assumed-role/user.name-platformengineer/environment-platformengineer",
					},
				},
				Spec: platformv1.JobRequestSpec{
					ContainerFrom: platformv1.JobRequestContainerFrom{
						PodSpecFrom: platformv1.JobRequestPodSpecFrom{
							Group: "apps/v1",
							Kind:  "Deployment",
							Name:  resourceName,
						},
						ContainerName: "foo-container",
					},
					Command: "echo",
					Args:    []string{"Hello, World!"},
				},
			}

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

			controllerReconciler := &JobRequestReviewReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: appsTypeNamespacedName,
			})

			Expect(err).NotTo(HaveOccurred())

			Expect(k8sClient.Get(ctx, appsTypeNamespacedName, jobRequestReview)).To(Succeed())
			Expect(jobRequestReview.Status.State).To(Equal("Rejected"))

			Expect(k8sClient.Get(ctx, appsTypeNamespacedName, jobRequest)).To(Succeed())
			Expect(jobRequest.Status.State).To(Equal("Rejected"))
			Expect(jobRequest.Status.ReviewName).To(Equal("test-resource"))
		})
	})
})
