//go:build e2e || smoke

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

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	eventsv1 "k8s.io/api/events/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/alphagov/govuk-job-request-operator/test/utils"
)

// serviceAccountName created for the project
const serviceAccountName = "govuk-job-request-operator-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "govuk-job-request-operator-controller-manager-metrics-service"

// namespace where the operator is deployed in
const controllerNamespace = "job-request-operator"

// users
const (
	jobRequestImpersonateUser = "arn:aws:sts::123456789012:assumed-role/job.req-developer/e2e"
	jobReviewImpersonateUser  = "arn:aws:sts::123456789012:assumed-role/job.rev-developer/e2e"
)

// fixtures
const (
	govukReplatformTestAppDeployment     = "govukReplatformTestApp.yaml"
	jobRequestForSuccessfulJob           = "jobRequestForSuccessfulJob.yaml"
	jobRequestWithAnnotation             = "jobRequestWithAnnotation.yaml"
	jobRequestWithoutAnnotation          = "jobRequestWithoutAnnotation.yaml"
	jobRequestForFailedJob               = "jobRequestForFailedJob.yaml"
	jobRequestForSecondJob               = "jobRequestForSecondJob.yaml"
	jobRequestReviewApproved             = "jobRequestReviewApproved.yaml"
	jobRequestReviewRejected             = "jobRequestReviewRejected.yaml"
	jobRequestReviewRejectedForSecondJob = "jobRequestReviewRejectedForSecondJob.yaml"
	jobRequestReviewWithAnnotation       = "jobRequestReviewWithAnnotation.yaml"
	jobRequestReviewWithoutAnnotation    = "jobRequestReviewWithoutAnnotation.yaml"
)

var _ = Describe("govuk-job-request-operator", Ordered, func() {
	var (
		controllerPodName string
	)

	BeforeAll(func(ctx context.Context) {
		By("creating a govuk-replatform-test-app Deployment")
		deploymentFixture, err := utils.RetrieveFixtureFilePath(govukReplatformTestAppDeployment)
		Expect(err).NotTo(HaveOccurred(), "Failed to retrieve deployment fixture filepath")

		cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", deploymentFixture, "-n", appNamespace)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create govuk-replatform-test-app deployment")

		By("waiting for the govuk-replatform-test-app deployment to become available.")
		verifyDeploymentInAvailableState := func(g Gomega) {
			cmd := exec.CommandContext(ctx, "kubectl", "get", "deployments", "govuk-replatform-test-app",
				"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}", "-n", appNamespace)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("True"), "govuk-replatform-test-app deployment not ready")
		}
		Eventually(ctx, verifyDeploymentInAvailableState).Should(Succeed())
	})

	AfterAll(func(ctx context.Context) {
		By("clean up Deployments")
		cmd := exec.CommandContext(ctx, "kubectl", "delete", "deployment", "--all", "-n", appNamespace)
		_, _ = utils.Run(cmd)

		By("clean up Events")
		cmd = exec.CommandContext(ctx, "kubectl", "delete", "events", "--all", "-n", appNamespace)
		_, _ = utils.Run(cmd)
	})

	AfterEach(func(ctx context.Context) {
		By("clean up JobReviews")
		cmd := exec.CommandContext(ctx, "kubectl", "delete", "jrr", "--all", "-n", appNamespace)
		_, _ = utils.Run(cmd)

		By("clean up JobRequests")
		cmd = exec.CommandContext(ctx, "kubectl", "delete", "jr", "--all", "-n", appNamespace)
		_, _ = utils.Run(cmd)

		By("clean up Events")
		cmd = exec.CommandContext(ctx, "kubectl", "delete", "events", "--all", "-n", appNamespace)
		_, _ = utils.Run(cmd)

		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.CommandContext(ctx, "kubectl", "logs", controllerPodName, "-n", controllerNamespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.CommandContext(ctx, "kubectl", "get", "events", "-n", controllerNamespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.CommandContext(ctx, "kubectl", "logs", "curl-metrics", "-n", controllerNamespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.CommandContext(ctx, "kubectl", "describe", "pod", controllerPodName, "-n", controllerNamespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)
	EnforceDefaultTimeoutsWhenUsingContexts()

	Context("when checking manager health", func() {
		It("should run successfully", func(ctx context.Context) {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				By("getting the name of the controller-manager pod")
				cmd := exec.CommandContext(ctx, "kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", controllerNamespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				By("validating the pod's status")
				cmd = exec.CommandContext(ctx, "kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", controllerNamespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(ctx, verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func(ctx context.Context) {
			By("validating that the metrics service is available")
			cmd := exec.CommandContext(ctx, "kubectl", "get", "service", metricsServiceName, "-n", controllerNamespace)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("ensuring the controller pod is ready")
			verifyControllerPodReady := func(g Gomega) {
				cmd := exec.CommandContext(ctx, "kubectl", "get", "pod", controllerPodName, "-n", controllerNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Controller pod not ready")
			}
			Eventually(ctx, verifyControllerPodReady, 3*time.Minute, time.Second).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.CommandContext(ctx, "kubectl", "logs", controllerPodName, "-n", controllerNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("Serving metrics server"),
					"Metrics server not yet started")
			}
			Eventually(ctx, verifyMetricsServerStarted, 3*time.Minute, time.Second).Should(Succeed())

			// +kubebuilder:scaffold:e2e-metrics-webhooks-readiness

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.CommandContext(ctx, "kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", controllerNamespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": [
								"for i in $(seq 1 30); do curl -v http://%s.%s.svc.cluster.local:8080/metrics && exit 0 || sleep 2; done; exit 1"
							],
							"securityContext": {
								"readOnlyRootFilesystem": true,
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccountName": "%s"
					}
				}`, metricsServiceName, controllerNamespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.CommandContext(ctx, "kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", controllerNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(ctx, verifyCurlUp).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			verifyMetricsAvailable := func(g Gomega) {
				metricsOutput, err := getMetricsOutput(ctx, controllerNamespace)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
				g.Expect(metricsOutput).NotTo(BeEmpty())
				g.Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
				g.Expect(metricsOutput).To(ContainSubstring("job_request_received_total"))
				g.Expect(metricsOutput).To(ContainSubstring("job_request_review_received_total"))
			}
			Eventually(ctx, verifyMetricsAvailable).Should(Succeed())
		})
	})

	Context("MutatingAdmissionPolicy", func() {
		It("Should add requested-by annotation to JobRequests", func(ctx context.Context) {
			Expect(createJobRequest(ctx, jobRequestWithoutAnnotation)).To(Succeed(), "Failed to create JobRequest")

			expectJobRequestToBeRequestedBy(ctx, "jr-govuk-replatform-test-app", jobRequestImpersonateUser)
		})

		It("Should override a user-specified requested-by annotation on JobRequests", func(ctx context.Context) {
			Expect(createJobRequest(ctx, jobRequestWithAnnotation)).To(Succeed(), "Failed to create JobRequest")

			expectJobRequestToBeRequestedBy(ctx, "jr-govuk-replatform-test-app", jobRequestImpersonateUser)
		})

		It("Should add reviewed-by annotation to JobRequestReviews", func(ctx context.Context) {
			Expect(createJobRequest(ctx, jobRequestWithAnnotation)).To(Succeed(), "Failed to create JobRequest")

			Expect(createJobRequestReview(ctx, jobRequestReviewWithoutAnnotation)).To(Succeed())

			expectJobRequestReviewToBeReviewedBy(ctx, "jrr-jr-govuk-replatform-test-app", jobReviewImpersonateUser)
		})

		It("Should override a user set reviewed-by annotation on JobRequestReviews", func(ctx context.Context) {
			Expect(createJobRequest(ctx, jobRequestWithAnnotation)).To(Succeed(), "Failed to create JobRequest")

			Expect(createJobRequestReview(ctx, jobRequestReviewWithAnnotation)).To(Succeed())

			expectJobRequestReviewToBeReviewedBy(ctx, "jrr-jr-govuk-replatform-test-app", jobReviewImpersonateUser)
		})
	})

	Context("when processing job requests", func() {
		Context("and there is another unreviewed JobRequest created before ours", func() {
			It("should set the correct JobRequest to Rejected if a JobRequestReview is created to reject it", func(ctx context.Context) {
				Expect(createJobRequest(ctx, jobRequestForSuccessfulJob)).To(Succeed(), "Failed to create JobRequest")
				expectJobRequestToBeInState(ctx, "jr-govuk-replatform-test-app", "Pending")

				Expect(createJobRequest(ctx, jobRequestForSecondJob)).To(Succeed(), "Failed to create second JobRequest")
				expectJobRequestToBeInState(ctx, "jr-govuk-replatform-test-app-2", "Pending")

				Expect(createJobRequestReview(ctx, jobRequestReviewRejectedForSecondJob)).To(Succeed())
				expectJobRequestReviewToBeInState(ctx, "jrr-jr-govuk-replatform-test-app-2", "Rejected")

				By("verifying the other JobRequestReview is still Pending")
				verifyOtherJobRequestStillInPendingState := func(g Gomega) {
					cmd := exec.CommandContext(ctx, "kubectl", "get", "jobrequests.platform.publishing.service.gov.uk", "jr-govuk-replatform-test-app",
						"-o", "jsonpath={.status.state}",
						"-n", appNamespace)
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(output).To(Equal("Pending"), "JobRequestReview reviewed the wrong JobRequest")
				}
				Consistently(ctx, verifyOtherJobRequestStillInPendingState, 5*time.Second, time.Second).Should(Succeed())

				expectJobRequestToBeInState(ctx, "jr-govuk-replatform-test-app-2", "Rejected")
			})
		})

		Context("and there is another already reviewed JobRequest, and our JobRequest is not yet reviewed", func() {
			BeforeEach(func(ctx context.Context) {
				Expect(createJobRequest(ctx, jobRequestForSuccessfulJob)).To(Succeed(), "Failed to create JobRequest")
				expectJobRequestToBeInState(ctx, "jr-govuk-replatform-test-app", "Pending")

				Expect(createJobRequestReview(ctx, jobRequestReviewApproved)).To(Succeed())
				expectJobRequestReviewToBeInState(ctx, "jrr-jr-govuk-replatform-test-app", "Approved")
			})

			It("should set the JobRequest to pending", func(ctx context.Context) {
				Expect(createJobRequest(ctx, jobRequestForSecondJob)).To(Succeed(), "Failed to create JobRequest")
				expectJobRequestToBeInState(ctx, "jr-govuk-replatform-test-app-2", "Pending")
			})
		})

		It("should create and successfully complete a job that is approved", func(ctx context.Context) {
			Expect(createJobRequest(ctx, jobRequestForSuccessfulJob)).To(Succeed(), "Failed to create JobRequest")
			expectJobRequestToBeInState(ctx, "jr-govuk-replatform-test-app", "Pending")

			expectResourceToHaveEventEmittedForState(ctx, "JobRequest", "Pending")

			Expect(createJobRequestReview(ctx, jobRequestReviewApproved)).To(Succeed())

			expectJobRequestReviewToBeInState(ctx, "jrr-jr-govuk-replatform-test-app", "Approved")
			expectResourceToHaveEventEmittedForState(ctx, "JobRequestReview", "Approved")
			expectResourceToHaveEventEmittedForState(ctx, "JobRequest", "Approved")

			expectJobRequestToBeInState(ctx, "jr-govuk-replatform-test-app", "Started")
			expectResourceToHaveEventEmittedForState(ctx, "JobRequest", "Started")

			By("Job successfully performs rake task")
			verifyJobCompleted := func(g Gomega) {
				cmd := exec.CommandContext(ctx, "kubectl", "get", "jobs", "jr-govuk-replatform-test-app",
					"-o", "jsonpath={.status.succeeded}",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"), "Job not succeeded")
			}

			Eventually(ctx, verifyJobCompleted).Should(Succeed())
			expectResourceToHaveEventEmittedForState(ctx, "JobRequest", "Complete")

			verifyJobOutput := func(g Gomega) {
				cmd := exec.CommandContext(ctx, "kubectl", "logs", "jobs/jr-govuk-replatform-test-app", "-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("Hello World!"))
			}

			Eventually(ctx, verifyJobOutput).Should(Succeed())
		})

		It("should create and successfully report a failed job that is approved", func(ctx context.Context) {
			Expect(createJobRequest(ctx, jobRequestForFailedJob)).To(Succeed(), "Failed to create JobRequest")
			expectJobRequestToBeInState(ctx, "jr-govuk-replatform-test-app", "Pending")
			expectResourceToHaveEventEmittedForState(ctx, "JobRequest", "Pending")

			Expect(createJobRequestReview(ctx, jobRequestReviewApproved)).To(Succeed())
			expectJobRequestReviewToBeInState(ctx, "jrr-jr-govuk-replatform-test-app", "Approved")

			expectResourceToHaveEventEmittedForState(ctx, "JobRequestReview", "Approved")
			expectResourceToHaveEventEmittedForState(ctx, "JobRequest", "Approved")

			expectJobRequestToBeInState(ctx, "jr-govuk-replatform-test-app", "Started")
			expectResourceToHaveEventEmittedForState(ctx, "JobRequest", "Started")

			By("Job fails to perform rake task")
			verifyJobFailed := func(g Gomega) {
				cmd := exec.CommandContext(ctx, "kubectl", "get", "jobs", "jr-govuk-replatform-test-app",
					"-o", "jsonpath={.status.failed}",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"), "Job not failed")
			}

			Eventually(ctx, verifyJobFailed).Should(Succeed())
			expectResourceToHaveEventEmittedForState(ctx, "JobRequest", "Failed")
		})

		It("should not create a job when JobRequest is rejected", func(ctx context.Context) {
			Expect(createJobRequest(ctx, jobRequestForSuccessfulJob)).To(Succeed(), "Failed to create JobRequest")

			expectJobRequestToBeInState(ctx, "jr-govuk-replatform-test-app", "Pending")
			expectResourceToHaveEventEmittedForState(ctx, "JobRequest", "Pending")

			Expect(createJobRequestReview(ctx, jobRequestReviewRejected)).To(Succeed())
			expectJobRequestReviewToBeInState(ctx, "jrr-jr-govuk-replatform-test-app", "Rejected")

			expectResourceToHaveEventEmittedForState(ctx, "JobRequestReview", "Rejected")
			expectResourceToHaveEventEmittedForState(ctx, "JobRequest", "Rejected")
			expectJobRequestToBeInState(ctx, "jr-govuk-replatform-test-app", "Rejected")

			By("Job not created")
			verifyJobNotStarted := func(g Gomega) {
				cmd := exec.CommandContext(ctx, "kubectl", "get", "jobs", "jr-govuk-replatform-test-app", "-n", appNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}

			Eventually(ctx, verifyJobNotStarted).Should(Succeed())
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks
	})
})

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput(ctx context.Context, controllerNamespace string) (string, error) {
	By("getting the curl-metrics logs")
	cmd := exec.CommandContext(ctx, "kubectl", "logs", "curl-metrics", "-n", controllerNamespace)
	return utils.Run(cmd)
}

func createJobRequest(ctx context.Context, jobRequestFixutrePath string) error {
	By(fmt.Sprintf("creating a JobRequest from fixture %s", jobRequestFixutrePath))
	jobRequestFixture, err := utils.RetrieveFixtureFilePath(jobRequestFixutrePath)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", jobRequestFixture, "-n", appNamespace, "--as", jobRequestImpersonateUser)
	_, err = utils.Run(cmd)

	return err
}

func createJobRequestReview(ctx context.Context, jobRequestReviewFixturePath string) error {
	By(fmt.Sprintf("creating a JobRequestReview form fixture %s", jobRequestReviewFixturePath))
	jobRequestReviewFixture, err := utils.RetrieveFixtureFilePath(jobRequestReviewFixturePath)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", jobRequestReviewFixture, "-n", appNamespace, "--as", jobReviewImpersonateUser)
	_, err = utils.Run(cmd)

	return err
}

func expectJobRequestToBeRequestedBy(ctx context.Context, jobRequestName, userArn string) {
	Eventually(ctx, func(g Gomega) {
		cmd := exec.CommandContext(ctx, "kubectl", "get", "jobrequests.platform.publishing.service.gov.uk", jobRequestName,
			"-o", "jsonpath={.metadata.annotations.platform\\.publishing\\.service\\.gov\\.uk/requested\\-by}",
			"-n", appNamespace)
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal(userArn))
	}).WithOffset(1).Should(Succeed())
}

func expectJobRequestReviewToBeReviewedBy(ctx context.Context, jobRequestReviewName, userArn string) {
	Eventually(ctx, func(g Gomega) {
		cmd := exec.CommandContext(ctx,
			"kubectl", "get", "jobrequestreviews.platform.publishing.service.gov.uk",
			jobRequestReviewName,
			"-o", "jsonpath={.metadata.annotations.platform\\.publishing\\.service\\.gov\\.uk/reviewed\\-by}",
			"-n", appNamespace,
		)
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal(userArn))
	}).WithOffset(1).Should(Succeed())
}

func expectJobRequestToBeInState(ctx context.Context, jobRequestName, state string) {
	Eventually(ctx, func(g Gomega) {
		cmd := exec.CommandContext(ctx,
			"kubectl", "get", "jobrequests.platform.publishing.service.gov.uk",
			jobRequestName,
			"-o", "jsonpath={.status.state}",
			"-n", appNamespace)
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal(state), "JobRequest in wrong status")
	}).WithOffset(1).Should(Succeed())
}

func expectJobRequestReviewToBeInState(ctx context.Context, jobRequestReviewName, state string) {
	Eventually(ctx, func(g Gomega) {
		cmd := exec.CommandContext(ctx,
			"kubectl", "get", "jobrequestreviews.platform.publishing.service.gov.uk",
			jobRequestReviewName,
			"-o", "jsonpath={.status.state}",
			"-n", appNamespace,
		)
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(output).To(Equal(state), "JobRequestReview in wrong status")
	}).WithOffset(1).Should(Succeed())
}

func expectResourceToHaveEventEmittedForState(ctx context.Context, resource, state string) {
	Eventually(ctx, func(g Gomega) {
		cmd := exec.CommandContext(ctx, "kubectl", "get", "events", "--field-selector",
			fmt.Sprintf("involvedObject.kind=%s,reason=%s", resource, state),
			"-o", "json",
			"-n", appNamespace)
		output, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())

		eventList := &eventsv1.EventList{}
		err = json.Unmarshal([]byte(output), &eventList)
		g.Expect(err).NotTo(HaveOccurred())

		g.Expect(eventList.Items).To(HaveLen(1))
		g.Expect(eventList.Items[0].Reason).To(Equal(state))
	}).WithOffset(1).Should(Succeed())
}
