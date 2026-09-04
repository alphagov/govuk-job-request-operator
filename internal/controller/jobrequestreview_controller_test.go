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
	"fmt"
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	log "sigs.k8s.io/controller-runtime/pkg/log"

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
				ResourceTtl:     defaultTestResourceTtl,
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

		BeforeEach(func() {
			By("verify events are empty")
			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(eventList.Items).To(BeEmpty())
			}, 1*time.Minute, 5*time.Second).Should(Succeed())
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

		AfterAll(func() {
			By("delete apps namespace")
			err := k8sClient.Delete(ctx, appsNamespace)
			Expect(err).NotTo(HaveOccurred())
			By("stop the manager")
			cancel()
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
				g.Expect(jobRequest.HasBeenApproved()).To(BeTrue())
				g.Expect(jobRequest.WasReviewedBy(jobRequestReview)).To(BeTrue())
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
				g.Expect(jobRequest.HasBeenRejected()).To(BeTrue())
				g.Expect(jobRequest.WasReviewedBy(jobRequestReview)).To(BeTrue())
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestReviewRejected)))
			}).Should(Succeed())
		})

		It("should go to Malformed state when the JobRequestReview has no reviewed-by annotation", func() {
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, "Rejected")
			delete(jobRequestReview.Annotations, "platform.publishing.service.gov.uk/reviewed-by")

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
				g.Expect(jobRequestReview.Status.State).To(Equal(platformv1.JobRequestReviewMalformed))
				g.Expect(jobRequest.HasBeenReviewed()).To(BeFalse())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestReviewMalformed)))
			}).Should(Succeed())
		})

		DescribeTable("when the JobRequestReview reviewed-by annotation is parsed",
			func(reviewedByAnnotation string, expectedJRRStatus platformv1.JobRequestReviewState, expectedJRStatus platformv1.JobRequestState) {
				By("Creating the JobRequest")
				jobRequest := jobRequestBuilder(jobRequestName, deploymentName, reviewNamespaceName, containerName)

				Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
				jobRequest.Status = platformv1.JobRequestStatus{
					State: platformv1.JobRequestPending,
				}
				Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

				By("Creating the JobRequestReview")
				jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, string(platformv1.JobRequestReviewRejected))
				jobRequestReview.Annotations[platformv1.JobRequestReviewReviewedByAnnotation] = reviewedByAnnotation
				Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
					g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
					g.Expect(jobRequestReview.Status.State).To(Equal(expectedJRRStatus))
					g.Expect(jobRequest.Status.State).To(Equal(expectedJRStatus))
				}).Should(Succeed())
			},
			Entry("when the reviewed-by annotation is not an ARN the JobRequestReivew should become Malformed",
				"wibble", platformv1.JobRequestReviewMalformed, platformv1.JobRequestPending),
			Entry("when the reviewed-by-annotation is not an assumed-role the JobRequestReivew should become Malformed",
				"arn:aws:sts::123456789012:user/joe.blogs", platformv1.JobRequestReviewMalformed, platformv1.JobRequestPending),
			Entry("when the reviewed-by-annotation is not a valid gds-users role or EntraID user the JobRequestReview should become Malformed",
				"arn:aws:sts::123456789012:assumed-role/foo/bar", platformv1.JobRequestReviewMalformed, platformv1.JobRequestPending),
			Entry("when the reviewed-by-annotation is a valid gds-users user it reviews the JobRequest",
				"arn:aws:sts::123456789012:assumed-role/joe.blogs-platformengineer/session-name", platformv1.JobRequestReviewRejected, platformv1.JobRequestRejected),
			Entry("when the reviewed-by-annotation is a valid EntraID user it reviews the JobRequest",
				"arn:aws:sts::123456789012:assumed-role/Developer/joe.blogs@dcms.gov.uk", platformv1.JobRequestReviewRejected, platformv1.JobRequestRejected),
		)

		DescribeTable("the JobRequestReview should go into Conflict state without reviewing the JobRequest if the reviewer is the same as the requester",
			func(requester string, reviewer string) {
				By("Creating the JobRequest")
				jobRequest := jobRequestBuilder(jobRequestName, deploymentName, reviewNamespaceName, containerName)
				jobRequest.Annotations[platformv1.JobRequestRequestedByAnnotation] = requester

				Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
				jobRequest.Status = platformv1.JobRequestStatus{
					State: platformv1.JobRequestPending,
				}
				Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

				By("Creating the JobRequestReview")
				jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, string(platformv1.JobRequestReviewRejected))
				jobRequestReview.Annotations[platformv1.JobRequestReviewReviewedByAnnotation] = reviewer
				Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
					g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
					g.Expect(jobRequestReview.Status.State).To(Equal(platformv1.JobRequestReviewConflict))
					g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestPending))
				}).Should(Succeed())
			},
			Entry(
				"with identical gds-users reviewer and requester",
				"arn:aws:sts::123456789012:assumed-role/joe.blogs-platformengineer/test-platformengineer",
				"arn:aws:sts::123456789012:assumed-role/joe.blogs-platformengineer/some-session-name",
			),
			Entry(
				"with same gds-users user but different role",
				"arn:aws:sts::123456789012:assumed-role/joe.blogs-platformengineer/test-platformengineer",
				"arn:aws:sts::123456789012:assumed-role/joe.blogs-developer/some-session-name",
			),
			Entry(
				"with identical EntraID reviewer and requester",
				"arn:aws:sts::123456789012:assumed-role/Developer/joe.blogs@dsit.gov.uk",
				"arn:aws:sts::123456789012:assumed-role/Developer/joe.blogs@dsit.gov.uk",
			),
			Entry(
				"with same EntraID user but different role",
				"arn:aws:sts::123456789012:assumed-role/Developer/joe.blogs@dsit.gov.uk",
				"arn:aws:sts::123456789012:assumed-role/Administrator/joe.blogs@dsit.gov.uk",
			),
		)

		DescribeTable("when the JobRequest has already been reviewed by another JobRequestReview",
			func(previousJobRequestReviewState platformv1.JobRequestReviewState, jobRequestState platformv1.JobRequestState) {
				By("Creating the Jobquest and the prior JobRequestReview")
				previousJobRequestReviewName := "previous-job-request-review"
				previousJobRequestReviewNamespaceName := types.NamespacedName{
					Name:      previousJobRequestReviewName,
					Namespace: reviewNamespaceName,
				}

				previousJobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, previousJobRequestReviewName, string(previousJobRequestReviewState))
				jobRequest := jobRequestBuilder(jobRequestName, deploymentName, reviewNamespaceName, containerName)

				By("Allowing the JobRequest to be reconciled with the prior JobRequestReview")
				Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
				jobRequest.Status = platformv1.JobRequestStatus{
					State: platformv1.JobRequestPending,
				}
				Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())
				Expect(k8sClient.Create(ctx, previousJobRequestReview)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, previousJobRequestReviewNamespaceName, previousJobRequestReview)).To(Succeed())
					g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
					g.Expect(string(jobRequest.Status.State)).To(Equal(string(previousJobRequestReviewState)))
					g.Expect(previousJobRequestReview.Status.State).To(Equal(previousJobRequestReviewState))
				}).Should(Succeed())

				By(fmt.Sprintf("Setting the State of the JobRequest to %s", jobRequestState))
				jobRequest.Status.State = jobRequestState
				Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
					g.Expect(jobRequest.Status.State).To(Equal(jobRequestState))
				}).Should(Succeed())

				By(fmt.Sprintf("Clearing all the Events in the %s namespace", reviewNamespaceName))
				Expect(k8sClient.DeleteAllOf(ctx, &eventsv1.Event{}, &client.DeleteAllOfOptions{
					DeleteOptions: client.DeleteOptions{
						GracePeriodSeconds: new(int64(0)),
						PropagationPolicy:  new(metav1.DeletePropagationBackground),
					},
					ListOptions: client.ListOptions{
						Namespace: reviewNamespaceName,
					},
				})).To(Succeed())

				jobRequestVersion := jobRequest.ResourceVersion

				By("Creating a new JobRequestReview which reviews the previous reviewed JobRequest")
				jobRequestReview := jobRequestReviewBuilder(jobRequestName, reviewNamespaceName, jobRequestReviewName, string(platformv1.JobRequestReviewRejected))
				Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())

				eventList := &eventsv1.EventList{}

				By("Waiting for the new JobRequestReview to go into a Conflict state")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
					g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())

					g.Expect(jobRequest.Status.State).To(Equal(jobRequestState))
					g.Expect(jobRequest.ResourceVersion).To(Equal(jobRequestVersion))
					g.Expect(jobRequest.WasReviewedBy(previousJobRequestReview)).To(BeTrue())
					g.Expect(jobRequestReview.Status.State).To(Equal(platformv1.JobRequestReviewConflict))

					g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
					g.Expect(eventList.Items).To(HaveLen(1))
					g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestReviewConflict)))
				}).Should(Succeed())
			},
			Entry("when the jobRequestReview was Rejected and the JobRequest is now in a Rejected state", platformv1.JobRequestReviewRejected, platformv1.JobRequestRejected),
			Entry("when the jobRequestReview was Approved and the JobRequest is now in a Approved state", platformv1.JobRequestReviewApproved, platformv1.JobRequestApproved),
			Entry("when the jobRequestReview was Accepted and the JobRequest is now in a Started state", platformv1.JobRequestReviewApproved, platformv1.JobRequestStarted),
			Entry("when the jobRequestReview was Accepted and the JobRequest is now in a Failed state", platformv1.JobRequestReviewApproved, platformv1.JobRequestFailed),
			Entry("when the jobRequestReview was Accepted and the JobRequest is now in a Complete state", platformv1.JobRequestReviewApproved, platformv1.JobRequestComplete),
		)
	})
})

var _ = Describe("JobRequestReview Pruning", Ordered, ContinueOnFailure, func() {
	ctx := context.Background()

	pruneNamespaceName := "apps-review-prune"
	deploymentName := "review-prune-deployment"
	containerName := "foo"
	jobRequestName := "review-prune-request"
	jobRequestReviewName := "review-prune-review"

	jobRequestNamespaceName := types.NamespacedName{
		Name:      jobRequestName,
		Namespace: pruneNamespaceName,
	}

	jobRequestReviewNamespaceName := types.NamespacedName{
		Name:      jobRequestReviewName,
		Namespace: pruneNamespaceName,
	}

	pruneNamespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pruneNamespaceName,
			Namespace: pruneNamespaceName,
		},
	}

	reconcilerWithTtl := func(resourceTtl time.Duration) *JobRequestReviewReconciler {
		return &JobRequestReviewReconciler{
			CacheClient:     k8sClient,
			ApiServerClient: k8sClient,
			Scheme:          k8sClient.Scheme(),
			Recorder:        events.NewFakeRecorder(10),
			Log:             log.Log,
			ResourceTtl:     resourceTtl,
		}
	}

	reconcile := func(reconciler *JobRequestReviewReconciler) (ctrl.Result, error) {
		return reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: jobRequestReviewNamespaceName})
	}

	createJobRequest := func(state platformv1.JobRequestState) {
		Expect(k8sClient.Create(ctx, deploymentBuilder(deploymentName, pruneNamespaceName))).To(Succeed())

		jobRequest := jobRequestBuilder(jobRequestName, deploymentName, pruneNamespaceName, containerName)
		Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

		if state != "" {
			jobRequest.Status.State = state
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())
		}
	}

	createJobRequestReview := func(decision string) {
		Expect(k8sClient.Create(ctx, jobRequestReviewBuilder(jobRequestName, pruneNamespaceName, jobRequestReviewName, decision))).To(Succeed())
	}

	// expectPruned reconciles until the JobRequestReview is older than
	// the resource TTL and has been deleted
	expectPruned := func() {
		reconciler := reconcilerWithTtl(100 * time.Millisecond)

		Eventually(func(g Gomega) {
			result, err := reconcile(reconciler)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result).To(Equal(ctrl.Result{}))

			jobRequestReview := &platformv1.JobRequestReview{}
			err = k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected the JobRequestReview to have been pruned")
		}).Should(Succeed())
	}

	BeforeAll(func() {
		By("creating apps-review-prune namespace")
		Expect(k8sClient.Create(ctx, pruneNamespace)).To(Succeed())
	})

	AfterEach(func() {
		var background metav1.DeletionPropagation = "Background"
		var graceSecs int64 = 0
		opts := &client.DeleteAllOfOptions{}
		opts.Namespace = pruneNamespaceName
		opts.GracePeriodSeconds = &graceSecs
		opts.PropagationPolicy = &background

		By("tearing down the JobRequestReviews")
		Expect(k8sClient.DeleteAllOf(ctx, &platformv1.JobRequestReview{}, opts)).To(Succeed())

		By("tearing down the JobRequests")
		Expect(k8sClient.DeleteAllOf(ctx, &platformv1.JobRequest{}, opts)).To(Succeed())

		By("tearing down the Deployments")
		Expect(k8sClient.DeleteAllOf(ctx, &appsv1.Deployment{}, opts)).To(Succeed())
	})

	AfterAll(func() {
		By("deleting apps-review-prune namespace")
		Expect(k8sClient.Delete(ctx, pruneNamespace)).To(Succeed())
	})

	It("should not prune a JobRequestReview that is younger than the resource TTL", func() {
		createJobRequest(platformv1.JobRequestPending)
		createJobRequestReview("Approved")

		reconciler := reconcilerWithTtl(defaultTestResourceTtl)

		result, err := reconcile(reconciler)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(ctrl.Result{}))

		By("verifying the JobRequestReview still exists and reconciliation continued")
		jobRequestReview := &platformv1.JobRequestReview{}
		Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
		Expect(jobRequestReview.DeletionTimestamp).To(BeNil())
		Expect(jobRequestReview.Status.State).To(Equal(platformv1.JobRequestReviewApproved))

		By("verifying the JobRequest was not deleted")
		Expect(k8sClient.Get(ctx, jobRequestNamespaceName, &platformv1.JobRequest{})).To(Succeed())
	})

	It("should prune a JobRequestReview that is older than the resource TTL", func() {
		createJobRequest(platformv1.JobRequestPending)
		createJobRequestReview("Approved")

		expectPruned()

		By("verifying the JobRequest was not deleted")
		Expect(k8sClient.Get(ctx, jobRequestNamespaceName, &platformv1.JobRequest{})).To(Succeed())
	})

	DescribeTable("should prune a JobRequestReview that is older than the resource TTL",
		func(decision string, jobRequestState platformv1.JobRequestState, jobRequestExists bool, expectedReviewState platformv1.JobRequestReviewState) {
			if jobRequestExists {
				createJobRequest(jobRequestState)
			}
			createJobRequestReview(decision)

			By("reconciling within the TTL so the JobRequestReview reaches its state")
			jobRequestReview := &platformv1.JobRequestReview{}
			Eventually(func(g Gomega) {
				_, err := reconcile(reconcilerWithTtl(defaultTestResourceTtl))
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(k8sClient.Get(ctx, jobRequestReviewNamespaceName, jobRequestReview)).To(Succeed())
				g.Expect(jobRequestReview.Status.State).To(Equal(expectedReviewState))
			}).Should(Succeed())

			expectPruned()

			if jobRequestExists {
				By("verifying the JobRequest was not deleted")
				Expect(k8sClient.Get(ctx, jobRequestNamespaceName, &platformv1.JobRequest{})).To(Succeed())
			}
		},
		Entry("when the JobRequestReview is Rejected",
			"Rejected", platformv1.JobRequestPending, true, platformv1.JobRequestReviewRejected),
		Entry("when the JobRequestReview is Approved",
			"Approved", platformv1.JobRequestPending, true, platformv1.JobRequestReviewApproved),
		Entry("when the JobRequest is Malformed",
			"Approved", platformv1.JobRequestMalformed, true, platformv1.JobRequestReviewMalformed),
		Entry("when the JobRequest is missing",
			"Approved", platformv1.JobRequestState(""), false, platformv1.JobRequestReviewNotFound),
	)
})
