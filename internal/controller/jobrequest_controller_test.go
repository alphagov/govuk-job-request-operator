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
	"maps"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	log "sigs.k8s.io/controller-runtime/pkg/log"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
)

const defaultTestResourceTtl = 720 * time.Hour

type PruneTestCase struct {
	InitialState  platformv1.JobRequestState
	ExpectedState platformv1.JobRequestState
	Reviewed      bool
	JobLaunched   bool
}

var _ = Describe("JobRequest Controller", Ordered, func() {
	Context("When reconciling a resource", func() {
		scheme := runtime.NewScheme()
		utilruntime.Must(clientgoscheme.AddToScheme(scheme))
		utilruntime.Must(platformv1.AddToScheme(scheme))
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
				Scheme: scheme,
			})
			Expect(err).ToNot(HaveOccurred())

			By("create the JobRequest controller")
			err = (&JobRequestReconciler{
				CacheClient:     mgr.GetClient(),
				ApiServerClient: mgr.GetAPIReader(),
				Scheme:          mgr.GetScheme(),
				Recorder:        mgr.GetEventRecorder("jobrequest-controller"),
				ResourceTtl:     defaultTestResourceTtl,
			}).SetupControllerWithManager(mgr)

			go func() {
				defer GinkgoRecover()
				err = mgr.Start(ctx)
				Expect(err).ToNot(HaveOccurred(), "failed to run manager")
			}()

			By("create apps namespace")
			Expect(k8sClient.Create(ctx, appsNamespace)).To(Succeed())
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
			var background metav1.DeletionPropagation = "Background"
			var graceSecs int64 = 0
			opts := &client.DeleteAllOfOptions{}
			opts.Namespace = appNamespaceName
			opts.GracePeriodSeconds = &graceSecs
			opts.PropagationPolicy = &background

			By("tearing down the Pods")
			Expect(k8sClient.DeleteAllOf(ctx, &v1.Pod{}, opts)).To(Succeed())

			By("tearing down the Jobs")
			Expect(k8sClient.DeleteAllOf(ctx, &batch.Job{}, opts)).To(Succeed())

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
			Expect(k8sClient.Delete(ctx, appsNamespace)).To(Succeed())
			By("stop the manager")
			cancel()
		})

		It("should successfully reconcile when JobRequest is 'Approved' and the job successfully runs", func() {
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, appNamespaceName, containerName)
			targetResource := deploymentBuilder(deploymentName, appNamespaceName)
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, appNamespaceName, jobRequestReviewName, "Approved")

			jobRequestStatus := platformv1.JobRequestStatus{
				State:      platformv1.JobRequestApproved,
				ReviewName: jobRequestReviewName,
			}

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestPending))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestPending)))
			}).Should(Succeed())

			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestStarted))
				g.Expect(jobRequest.Status.JobName).To(Equal(jobRequestName))
				g.Expect(eventList.Items).To(HaveLen(3))
				g.Expect(eventList.Items[1].Reason).To(Equal(string(platformv1.JobRequestApproved)))
				g.Expect(eventList.Items[2].Reason).To(Equal(string(platformv1.JobRequestStarted)))
			}).Should(Succeed())

			jobList := &batch.JobList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, jobList, jobOpts...)).To(Succeed())
				g.Expect(jobList.Items).To(HaveLen(1))
				g.Expect(jobList.Items[0].GetName()).To(Equal(jobRequestName))
				g.Expect(jobList.Items[0].GetNamespace()).To(Equal(appNamespaceName))
				g.Expect(jobList.Items[0].Spec.BackoffLimit).To(Equal(ptr.To(int32(0))))
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

			startTime := metav1.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			completionTime := metav1.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)

			jobStatus := batch.JobStatus{
				StartTime:      &startTime,
				CompletionTime: &completionTime,
				Succeeded:      1,
				Conditions: []batch.JobCondition{{
					Type:   batch.JobSuccessCriteriaMet,
					Status: v1.ConditionTrue,
				}, {
					Type:   batch.JobComplete,
					Status: v1.ConditionTrue,
				}},
			}

			job := jobList.Items[0]
			job.Status = jobStatus
			Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestComplete))
				g.Expect(jobRequest.Status.JobName).To(Equal(jobRequestName))
				g.Expect(eventList.Items).To(HaveLen(4))
				g.Expect(eventList.Items[3].Reason).To(Equal(string(platformv1.JobRequestComplete)))
			}).Should(Succeed())
		})

		It("should successfully reconcile when JobRequest is 'Started' and there is no job", func() {
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, appNamespaceName, containerName)
			targetResource := deploymentBuilder(deploymentName, appNamespaceName)
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, appNamespaceName, jobRequestReviewName, "Approved")

			jobRequestStatus := platformv1.JobRequestStatus{
				State:      platformv1.JobRequestStarted,
				ReviewName: jobRequestReviewName,
			}

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestPending))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestPending)))
			}).Should(Succeed())

			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestMalformed))
				g.Expect(eventList.Items).To(HaveLen(2))
				g.Expect(eventList.Items[1].Reason).To(Equal(string(platformv1.JobRequestMalformed)))
			}).Should(Succeed())
		})

		It("should successfully reconcile when JobRequest is 'Approved' no new job is created when it already exists", func() {
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, appNamespaceName, containerName)
			targetResource := deploymentBuilder(deploymentName, appNamespaceName)
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, appNamespaceName, jobRequestReviewName, "Approved")

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestPending))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestPending)))
			}).Should(Succeed())

			job := &batch.Job{}
			job.Labels = make(map[string]string)
			job.Annotations = make(map[string]string)
			job.Name = jobRequest.Name
			job.Namespace = targetResource.Namespace
			jobTemplatePodSpec := *targetResource.Spec.Template.DeepCopy()
			jobTemplatePodSpec.Spec.Containers = targetResource.Spec.Template.Spec.Containers
			jobTemplatePodSpec.Spec.RestartPolicy = v1.RestartPolicyNever
			job.Spec.Template = jobTemplatePodSpec
			job.Spec.BackoffLimit = ptr.To(int32(0))
			maps.Copy(job.Annotations, targetResource.Annotations)
			maps.Copy(job.Labels, targetResource.Labels)
			_ = ctrl.SetControllerReference(jobRequest, job, scheme)

			jobRequestStatus := platformv1.JobRequestStatus{
				State:      platformv1.JobRequestApproved,
				ReviewName: jobRequestReviewName,
			}

			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())
			Expect(k8sClient.Create(ctx, job)).To(Succeed())

			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

			jobList := &batch.JobList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, jobList, jobOpts...)).To(Succeed())
				g.Expect(jobList.Items).To(HaveLen(1))
			}).Should(Succeed())
		})

		It("should successfully reconcile when JobRequest is 'Approved' and the job is in a failed state", func() {
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, appNamespaceName, containerName)
			targetResource := deploymentBuilder(deploymentName, appNamespaceName)
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, appNamespaceName, jobRequestReviewName, "Approved")

			jobRequestStatus := platformv1.JobRequestStatus{
				State:      platformv1.JobRequestApproved,
				ReviewName: jobRequestReviewName,
			}

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestPending))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestPending)))
			}).Should(Succeed())

			Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestStarted))
				g.Expect(jobRequest.Status.JobName).To(Equal(jobRequestName))
				g.Expect(eventList.Items).To(HaveLen(3))
				g.Expect(eventList.Items[1].Reason).To(Equal(string(platformv1.JobRequestApproved)))
				g.Expect(eventList.Items[2].Reason).To(Equal(string(platformv1.JobRequestStarted)))
			}).Should(Succeed())

			jobList := &batch.JobList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.List(ctx, jobList, jobOpts...)).To(Succeed())
				g.Expect(jobList.Items).To(HaveLen(1))
				g.Expect(jobList.Items[0].GetName()).To(Equal(jobRequestName))
				g.Expect(jobList.Items[0].GetNamespace()).To(Equal(appNamespaceName))
				g.Expect(jobList.Items[0].Spec.BackoffLimit).To(Equal(ptr.To(int32(0))))
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

			startTime := metav1.Now()

			jobStatus := batch.JobStatus{
				StartTime: &startTime,
				Failed:    1,
				Conditions: []batch.JobCondition{{
					Type:   batch.JobFailureTarget,
					Status: v1.ConditionTrue,
				}, {
					Type:   batch.JobFailed,
					Status: v1.ConditionTrue,
				}},
			}

			job := jobList.Items[0]
			job.Status = jobStatus
			Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestFailed))
				g.Expect(jobRequest.Status.JobName).To(Equal(jobRequestName))
				g.Expect(eventList.Items).To(HaveLen(4))
				g.Expect(eventList.Items[3].Reason).To(Equal(string(platformv1.JobRequestFailed)))
			}).Should(Succeed())
		})

		It("should successfully reconcile if we cannot retrieve the target resource in the JobRequest from the cluster and the job should not be created", func() {
			jobRequest := jobRequestBuilder(jobRequestName, "example-app", appNamespaceName, "example-container")
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestMalformed))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestMalformed)))
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
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestMalformed))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestMalformed)))
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
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestPending))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestPending)))
			}).Should(Succeed())
		})

		It("should successfully reconcile when JobRequest is 'Rejected' and the job should not be created", func() {
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, appNamespaceName, containerName)
			targetResource := deploymentBuilder(deploymentName, appNamespaceName)
			jobRequestReview := jobRequestReviewBuilder(jobRequestName, appNamespaceName, jobRequestReviewName, "Rejected")

			jobRequestStatus := platformv1.JobRequestStatus{
				State:      platformv1.JobRequestRejected,
				ReviewName: "test",
			}

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestPending))
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestPending)))
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
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestRejected))
				g.Expect(eventList.Items).To(HaveLen(2))
				g.Expect(eventList.Items[1].Reason).To(Equal(string(platformv1.JobRequestRejected)))
			}).Should(Succeed())
		})

		It("should successfully reconcile with no job created when JobRequest is 'Malformed'", func() {
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, appNamespaceName, containerName)
			targetResource := deploymentBuilder(jobRequestName, appNamespaceName)
			jobRequestReview := jobRequestReviewBuilder(deploymentName, appNamespaceName, jobRequestReviewName, "Approved")

			jobRequestStatus := platformv1.JobRequestStatus{
				State: platformv1.JobRequestMalformed,
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
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestMalformed))
			}).Should(Succeed())
		})

		It("should go to Malformed state when the JobRequest has no requested-by annotation", func() {
			jobRequest := jobRequestBuilder(jobRequestName, deploymentName, appNamespaceName, containerName)
			delete(jobRequest.Annotations, "platform.publishing.service.gov.uk/requested-by")

			targetResource := deploymentBuilder(jobRequestName, appNamespaceName)

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			eventList := &eventsv1.EventList{}

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
				g.Expect(jobRequest.Status.State).To(Equal(platformv1.JobRequestMalformed))
				g.Expect(k8sClient.List(ctx, eventList, eventOpts...)).To(Succeed())
				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal(string(platformv1.JobRequestMalformed)))
			}).Should(Succeed())

			jobList := &batch.JobList{}

			Expect(k8sClient.List(ctx, jobList, jobOpts...)).To(Succeed())
			Expect(jobList.Items).To(BeEmpty())
		})

		DescribeTable("when the JobRequest requested-by annotation is parsed",
			func(requestedByAnnotation string, expectedJRStatus platformv1.JobRequestState) {
				By("Creating a deployment to target")
				targetResource := deploymentBuilder(deploymentName, appNamespaceName)
				Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())

				By("Creating the JobRequest")
				jobRequest := jobRequestBuilder(jobRequestName, deploymentName, appNamespaceName, containerName)
				jobRequest.Annotations[platformv1.JobRequestRequestedByAnnotation] = requestedByAnnotation
				Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
					g.Expect(jobRequest.Status.State).To(Equal(expectedJRStatus))
				}).Should(Succeed())
			},
			Entry("when the requested-by annotation is not an ARN the JobRequest should become Malformed",
				"wibble", platformv1.JobRequestMalformed),
			Entry("when the requested-by-annotation is not an assumed-role the JobRequest should become Malformed",
				"arn:aws:sts::123456789012:user/joe.blogs", platformv1.JobRequestMalformed),
			Entry("when the requested-by-annotation is not a valid gds-users role or EntraID user the JobRequest should become Malformed",
				"arn:aws:sts::123456789012:assumed-role/foo/bar", platformv1.JobRequestMalformed),
			Entry("when the requested-by-annotation is a valid gds-users user it creates the JobRequest",
				"arn:aws:sts::123456789012:assumed-role/joe.blogs-platformengineer/session-name", platformv1.JobRequestPending),
			Entry("when the requested-by-annotation is a valid EntraID user it creates the JobRequest",
				"arn:aws:sts::123456789012:assumed-role/Developer/joe.blogs@dcms.gov.uk", platformv1.JobRequestPending),
		)
	})
})

var _ = Describe("JobRequest Pruning", Ordered, ContinueOnFailure, func() {
	ctx := context.Background()

	pruneNamespaceName := "apps-prune"
	deploymentName := "deployment"
	containerName := "foo"
	jobRequestName := "request"

	jobRequestNamespaceName := types.NamespacedName{
		Name:      jobRequestName,
		Namespace: pruneNamespaceName,
	}

	pruneNamespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pruneNamespaceName,
			Namespace: pruneNamespaceName,
		},
	}

	reconcilerWithTtl := func(resourceTtl time.Duration) *JobRequestReconciler {
		return &JobRequestReconciler{
			CacheClient:     k8sClient,
			ApiServerClient: k8sClient,
			Scheme:          k8sClient.Scheme(),
			Recorder:        events.NewFakeRecorder(10),
			Log:             log.Log,
			ResourceTtl:     resourceTtl,
		}
	}

	reconcile := func(reconciler *JobRequestReconciler) (ctrl.Result, error) {
		return reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: jobRequestNamespaceName})
	}

	var deployment *appsv1.Deployment

	BeforeAll(func() {
		By("creating apps-prune namespace")
		Expect(k8sClient.Create(ctx, pruneNamespace)).To(Succeed())
	})

	BeforeEach(func() {
		By("creating the target resource")
		deployment = deploymentBuilder(deploymentName, pruneNamespaceName)
		Expect(k8sClient.Create(ctx, deployment)).To(Succeed())
	})

	AfterEach(func() {
		var background metav1.DeletionPropagation = "Background"
		var graceSecs int64 = 0
		opts := &client.DeleteAllOfOptions{}
		opts.Namespace = pruneNamespaceName
		opts.GracePeriodSeconds = &graceSecs
		opts.PropagationPolicy = &background

		By("tearing down the JobRequests")
		Expect(k8sClient.DeleteAllOf(ctx, &platformv1.JobRequest{}, opts)).To(Succeed())

		By("tearing down the JobRequestReviews")
		Expect(k8sClient.DeleteAllOf(ctx, &platformv1.JobRequestReview{}, opts)).To(Succeed())

		By("tearing down the Pods")
		Expect(k8sClient.DeleteAllOf(ctx, &v1.Pod{}, opts)).To(Succeed())

		By("tearing down the Jobs")
		Expect(k8sClient.DeleteAllOf(ctx, &batch.Job{}, opts)).To(Succeed())

		By("tearing down the Deployments")
		Expect(k8sClient.DeleteAllOf(ctx, &appsv1.Deployment{}, opts)).To(Succeed())
	})

	AfterAll(func() {
		By("deleting apps-prune namespace")
		Expect(k8sClient.Delete(ctx, pruneNamespace)).To(Succeed())
	})

	for _, pruneTestCase := range []PruneTestCase{
		{InitialState: platformv1.JobRequestPending, ExpectedState: platformv1.JobRequestPending, Reviewed: false, JobLaunched: false},
		{InitialState: platformv1.JobRequestApproved, ExpectedState: platformv1.JobRequestStarted, Reviewed: true, JobLaunched: false},
		{InitialState: platformv1.JobRequestRejected, ExpectedState: platformv1.JobRequestRejected, Reviewed: true, JobLaunched: false},
		{InitialState: platformv1.JobRequestStarted, ExpectedState: platformv1.JobRequestStarted, Reviewed: true, JobLaunched: true},
		{InitialState: platformv1.JobRequestComplete, ExpectedState: platformv1.JobRequestComplete, Reviewed: true, JobLaunched: true},
		{InitialState: platformv1.JobRequestFailed, ExpectedState: platformv1.JobRequestFailed, Reviewed: true, JobLaunched: false},
		{InitialState: platformv1.JobRequestMalformed, ExpectedState: platformv1.JobRequestMalformed, Reviewed: false, JobLaunched: false},
	} {

		It(fmt.Sprintf("should not prune a %s JobRequest that is younger than the resource TTL", pruneTestCase.InitialState), func() {
			By("Creating the reconciler and reconciling the resource")
			reconciler := reconcilerWithTtl(defaultTestResourceTtl)

			By(fmt.Sprintf("creating the JobRequest in a %s state", pruneTestCase.InitialState))
			jobRequest := SetupJobRequestForPruneTest(
				ctx, pruneTestCase, jobRequestName, deploymentName, pruneNamespaceName, containerName, deployment, reconciler.Scheme,
			)

			result, err := reconcile(reconciler)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			By("verifying the JobRequest still exists and reconciliation continued")
			Expect(k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)).To(Succeed())
			Expect(jobRequest.DeletionTimestamp).To(BeNil())
			Expect(jobRequest.Status.State).To(Equal(pruneTestCase.ExpectedState))
		})

		It(fmt.Sprintf("should prune a %s JobRequest that is older than the resource TTL", pruneTestCase.InitialState), func() {
			By("Creating the reconciler and reconciling the resource")
			reconciler := reconcilerWithTtl(100 * time.Millisecond)

			By(fmt.Sprintf("creating the JobRequest in a %s state", pruneTestCase.InitialState))
			jobRequest := SetupJobRequestForPruneTest(
				ctx, pruneTestCase, jobRequestName, deploymentName, pruneNamespaceName, containerName, deployment, reconciler.Scheme,
			)

			By("reconciling until the JobRequest is older than the resource TTL")
			Eventually(func(g Gomega) {
				result, err := reconcile(reconciler)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(ctrl.Result{}))

				err = k8sClient.Get(ctx, jobRequestNamespaceName, jobRequest)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected the JobRequest to have been pruned")
			}).Should(Succeed())
		})
	}
})

func SetupJobRequestForPruneTest(ctx context.Context, pruneTestCase PruneTestCase, requestName, deploymentName, namespaceName, containerName string, targetResource *appsv1.Deployment, scheme *runtime.Scheme) *platformv1.JobRequest {
	jobRequest := jobRequestBuilder(requestName, deploymentName, namespaceName, containerName)
	Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

	if pruneTestCase.Reviewed {
		By("creating the JobRequestReview")
		var reviewDecision string
		if pruneTestCase.InitialState == platformv1.JobRequestRejected {
			reviewDecision = string(platformv1.JobRequestReviewRejected)
		} else {
			reviewDecision = string(platformv1.JobRequestReviewApproved)
		}
		jobRequestReview := jobRequestReviewBuilder(requestName, namespaceName, "review", reviewDecision)
		Expect(k8sClient.Create(ctx, jobRequestReview)).To(Succeed())
	}

	if pruneTestCase.JobLaunched {
		By("creating the Job")
		job := &batch.Job{}
		job.Labels = make(map[string]string)
		job.Annotations = make(map[string]string)
		job.Name = requestName
		job.Namespace = namespaceName
		jobTemplatePodSpec := *targetResource.Spec.Template.DeepCopy()
		jobTemplatePodSpec.Spec.Containers = targetResource.Spec.Template.Spec.Containers
		jobTemplatePodSpec.Spec.RestartPolicy = v1.RestartPolicyNever
		job.Spec.Template = jobTemplatePodSpec
		job.Spec.BackoffLimit = ptr.To(int32(0))
		maps.Copy(job.Annotations, targetResource.Annotations)
		maps.Copy(job.Labels, targetResource.Labels)
		_ = ctrl.SetControllerReference(jobRequest, job, scheme)
		Expect(k8sClient.Create(ctx, job)).To(Succeed())
	}

	By(fmt.Sprintf("updating the JobRequest to the %s state", pruneTestCase.InitialState))
	jobRequest.Status = platformv1.JobRequestStatus{State: pruneTestCase.InitialState}
	if pruneTestCase.Reviewed {
		jobRequest.Status.ReviewName = "review"
	}
	if pruneTestCase.JobLaunched {
		jobRequest.Status.JobName = "job"
	}
	Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

	return jobRequest
}
