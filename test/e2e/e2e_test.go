//go:build e2e
// +build e2e

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

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	eventsv1 "k8s.io/api/events/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/alphagov/govuk-job-request-operator/test/utils"
)

// namespace where the operator is deployed in
const controllerNamespace = "govuk-job-request-operator-system"

// namespace where resources are deployed in
const appNamespace = "apps"

// serviceAccountName created for the project
const serviceAccountName = "govuk-job-request-operator-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "govuk-job-request-operator-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "govuk-job-request-operator-metrics-binding"

// fixtures
const govukReplatformTestAppDeployment = "govukReplatformTestApp.yaml"
const jobRequestForSuccessfulJob = "jobRequestForSuccessfulJob.yaml"
const jobRequestForFailedJob = "jobRequestForFailedJob.yaml"
const jobRequestReviewApproved = "jobRequestReviewApproved.yaml"
const jobRequestReviewRejected = "jobRequestReviewRejected.yaml"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", controllerNamespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", controllerNamespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", managerImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")

		By("creating apps namespace")
		cmd = exec.Command("kubectl", "create", "ns", appNamespace)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create apps namespace")

		By("labeling the apps namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", appNamespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label apps namespace with restricted policy")
	})

	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", controllerNamespace)
		_, _ = utils.Run(cmd)

		By("removing apps namespace")
		cmd = exec.Command("kubectl", "delete", "ns", appNamespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", controllerNamespace)
		_, _ = utils.Run(cmd)
	})

	BeforeEach(func() {
		By("clean up JobReviews")
		cmd := exec.Command("kubectl", "delete", "jrr", "--all", "-n", appNamespace)
		_, _ = utils.Run(cmd)

		By("clean up JobRequests")
		cmd = exec.Command("kubectl", "delete", "jr", "--all", "-n", appNamespace)
		_, _ = utils.Run(cmd)

		By("clean up Deployments")
		cmd = exec.Command("kubectl", "delete", "deployment", "--all", "-n", appNamespace)
		_, _ = utils.Run(cmd)

		By("clean up Events")
		cmd = exec.Command("kubectl", "delete", "events", "--all", "-n", appNamespace)
		_, _ = utils.Run(cmd)
	})

	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", controllerNamespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", controllerNamespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", controllerNamespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", controllerNamespace)
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

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				By("getting the name of the controller-manager pod")
				cmd := exec.Command("kubectl", "get",
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
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", controllerNamespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=govuk-job-request-operator-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", controllerNamespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", controllerNamespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("ensuring the controller pod is ready")
			verifyControllerPodReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pod", controllerPodName, "-n", controllerNamespace,
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "Controller pod not ready")
			}
			Eventually(verifyControllerPodReady, 3*time.Minute, time.Second).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", controllerNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("Serving metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted, 3*time.Minute, time.Second).Should(Succeed())

			// +kubebuilder:scaffold:e2e-metrics-webhooks-readiness

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
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
								"for i in $(seq 1 30); do curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics && exit 0 || sleep 2; done; exit 1"
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
				}`, token, metricsServiceName, controllerNamespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", controllerNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			verifyMetricsAvailable := func(g Gomega) {
				metricsOutput, err := getMetricsOutput()
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
				g.Expect(metricsOutput).NotTo(BeEmpty())
				g.Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
			}
			Eventually(verifyMetricsAvailable).Should(Succeed())
		})

		It("should create and successfully complete a job that is approved", func() {
			By("creating a govuk-replatform-test-app Deployment for the JobRequest to run a rake task from")

			deploymentFixture, err := utils.RetrieveFixtureFilePath(govukReplatformTestAppDeployment)
			Expect(err).NotTo(HaveOccurred(), "Failed to retrieve deployment fixture filepath")

			cmd := exec.Command("kubectl", "apply", "-f", deploymentFixture, "-n", appNamespace)

			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create govuk-replatform-test-app deployment")

			By("waiting for the govuk-replatform-test-app deployment to become available.")
			verifyDeploymentInAvailableState := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployments", "govuk-replatform-test-app",
					"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}", "-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "govuk-replatform-test-app deployment not ready")
			}
			Eventually(verifyDeploymentInAvailableState).Should(Succeed())

			By("creating a JobRequest")
			jobRequestFixture, err := utils.RetrieveFixtureFilePath(jobRequestForSuccessfulJob)
			Expect(err).NotTo(HaveOccurred(), "Failed to retrieve current working directory")

			cmd = exec.Command("kubectl", "apply", "-f", jobRequestFixture, "-n", appNamespace)

			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create govuk-replatform-test-app jobRequest")

			verifyJobRequestInPendingState := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "jobrequests.platform.publishing.service.gov.uk", "govuk-replatform-test-app",
					"-o", "jsonpath={.status.state}",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Pending"), "JobRequest in wrong status")
			}

			verifyJobRequestPendingEventEmitted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "events", "--field-selector",
					"involvedObject.kind=JobRequest,reason=Pending",
					"-o", "json",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				eventList := &eventsv1.EventList{}
				err = json.Unmarshal([]byte(output), &eventList)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Pending"))
			}

			Eventually(verifyJobRequestInPendingState).Should(Succeed())
			Eventually(verifyJobRequestPendingEventEmitted).Should(Succeed())

			By("creating a JobRequestReview to approve the JobRequest")
			jobRequestReviewFixture, err := utils.RetrieveFixtureFilePath(jobRequestReviewApproved)
			Expect(err).NotTo(HaveOccurred(), "Failed to retrieve current working directory")

			cmd = exec.Command("kubectl", "apply", "-f", jobRequestReviewFixture, "-n", appNamespace)

			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create govuk-replatform-test-app jobRequestReviewApproved")

			By("JobRequestReview is in Approved state")
			verifyJobRequestReviewInApprovedState := func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "jobrequestreviews.platform.publishing.service.gov.uk", "govuk-replatform-test-app",
					"-o", "jsonpath={.status.state}",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Approved"), "JobRequestReview in wrong status")
			}

			verifyJobRequestReviewApprovedEventEmitted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "events", "--field-selector",
					"involvedObject.kind=JobRequestReview,reason=Approved",
					"-o", "json",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				eventList := &eventsv1.EventList{}
				err = json.Unmarshal([]byte(output), &eventList)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Approved"))
			}

			verifyJobRequestApprovedEventEmitted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "events", "--field-selector",
					"involvedObject.kind=JobRequest,reason=Approved",
					"-o", "json",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				eventList := &eventsv1.EventList{}
				err = json.Unmarshal([]byte(output), &eventList)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Approved"))
			}

			Eventually(verifyJobRequestReviewInApprovedState).Should(Succeed())
			Eventually(verifyJobRequestReviewApprovedEventEmitted).Should(Succeed())
			Eventually(verifyJobRequestApprovedEventEmitted).Should(Succeed())

			By("JobRequest is in Started state")
			verifyJobRequestStarted := func(g Gomega) {
				jobRequestStateJSON := `{"jobName":"govuk-replatform-test-app","reviewName":"govuk-replatform-test-app","state":"Started"}`
				cmd = exec.Command("kubectl", "get", "jobrequests.platform.publishing.service.gov.uk", "govuk-replatform-test-app",
					"-o", "jsonpath={.status}",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).Should(MatchJSON(jobRequestStateJSON))
			}

			verifyJobRequestStartedEventEmitted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "events", "--field-selector",
					"involvedObject.kind=JobRequest,reason=Started",
					"-o", "json",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				eventList := &eventsv1.EventList{}
				err = json.Unmarshal([]byte(output), &eventList)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Started"))
			}

			Eventually(verifyJobRequestStarted).Should(Succeed())
			Eventually(verifyJobRequestStartedEventEmitted).Should(Succeed())

			By("Job successfully performs rake task")
			verifyJobCompleted := func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "jobs", "govuk-replatform-test-app",
					"-o", "jsonpath={.status.succeeded}",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"), "Job not succeeded")
			}
			verifyJobRequestCompleteEventEmitted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "events", "--field-selector",
					"involvedObject.kind=JobRequest,reason=Complete",
					"-o", "json",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				eventList := &eventsv1.EventList{}
				err = json.Unmarshal([]byte(output), &eventList)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Complete"))
			}

			Eventually(verifyJobCompleted).Should(Succeed())
			Eventually(verifyJobRequestCompleteEventEmitted).Should(Succeed())

			verifyJobOutput := func(g Gomega) {
				cmd = exec.Command("kubectl", "logs", "jobs/govuk-replatform-test-app", "-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("Hello World!"))
			}

			Eventually(verifyJobOutput).Should(Succeed())
		})

		It("should create and successfully report a failed job that is approved", func() {
			By("creating a govuk-replatform-test-app Deployment for the JobRequest to run a rake task from")

			deploymentFixture, err := utils.RetrieveFixtureFilePath(govukReplatformTestAppDeployment)
			Expect(err).NotTo(HaveOccurred(), "Failed to retrieve deployment fixture filepath")

			cmd := exec.Command("kubectl", "apply", "-f", deploymentFixture, "-n", appNamespace)

			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create govuk-replatform-test-app deployment")

			By("waiting for the govuk-replatform-test-app deployment to become available.")
			verifyDeploymentInAvailableState := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployments", "govuk-replatform-test-app",
					"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}", "-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "govuk-replatform-test-app deployment not ready")
			}
			Eventually(verifyDeploymentInAvailableState).Should(Succeed())

			By("creating a JobRequest")
			jobRequestFixture, err := utils.RetrieveFixtureFilePath(jobRequestForFailedJob)
			Expect(err).NotTo(HaveOccurred(), "Failed to retrieve current working directory")

			cmd = exec.Command("kubectl", "apply", "-f", jobRequestFixture, "-n", appNamespace)

			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create govuk-replatform-test-app jobRequest")

			verifyJobRequestInPendingState := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "jobrequests.platform.publishing.service.gov.uk", "govuk-replatform-test-app",
					"-o", "jsonpath={.status.state}",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Pending"), "JobRequest in wrong status")
			}

			verifyJobRequestPendingEventEmitted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "events", "--field-selector",
					"involvedObject.kind=JobRequest,reason=Pending",
					"-o", "json",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				eventList := &eventsv1.EventList{}
				err = json.Unmarshal([]byte(output), &eventList)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Pending"))
			}

			Eventually(verifyJobRequestInPendingState).Should(Succeed())
			Eventually(verifyJobRequestPendingEventEmitted).Should(Succeed())

			By("creating a JobRequestReview to approve the JobRequest")
			jobRequestReviewFixture, err := utils.RetrieveFixtureFilePath(jobRequestReviewApproved)
			Expect(err).NotTo(HaveOccurred(), "Failed to retrieve current working directory")

			cmd = exec.Command("kubectl", "apply", "-f", jobRequestReviewFixture, "-n", appNamespace)

			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create govuk-replatform-test-app jobRequestReviewApproved")

			By("JobRequestReview is in Approved state")
			verifyJobRequestReviewInApprovedState := func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "jobrequestreviews.platform.publishing.service.gov.uk", "govuk-replatform-test-app",
					"-o", "jsonpath={.status.state}",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Approved"), "JobRequestReview in wrong status")
			}

			verifyJobRequestReviewApprovedEventEmitted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "events", "--field-selector",
					"involvedObject.kind=JobRequestReview,reason=Approved",
					"-o", "json",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				eventList := &eventsv1.EventList{}
				err = json.Unmarshal([]byte(output), &eventList)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Approved"))
			}

			verifyJobRequestApprovedEventEmitted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "events", "--field-selector",
					"involvedObject.kind=JobRequest,reason=Approved",
					"-o", "json",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				eventList := &eventsv1.EventList{}
				err = json.Unmarshal([]byte(output), &eventList)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Approved"))
			}

			Eventually(verifyJobRequestReviewInApprovedState).Should(Succeed())
			Eventually(verifyJobRequestReviewApprovedEventEmitted).Should(Succeed())
			Eventually(verifyJobRequestApprovedEventEmitted).Should(Succeed())

			By("JobRequest is in Started state")
			verifyJobRequestStarted := func(g Gomega) {
				jobRequestStateJSON := `{"jobName":"govuk-replatform-test-app","reviewName":"govuk-replatform-test-app","state":"Started"}`
				cmd = exec.Command("kubectl", "get", "jobrequests.platform.publishing.service.gov.uk", "govuk-replatform-test-app",
					"-o", "jsonpath={.status}",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).Should(MatchJSON(jobRequestStateJSON))
			}

			verifyJobRequestStartedEventEmitted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "events", "--field-selector",
					"involvedObject.kind=JobRequest,reason=Started",
					"-o", "json",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				eventList := &eventsv1.EventList{}
				err = json.Unmarshal([]byte(output), &eventList)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Started"))
			}

			Eventually(verifyJobRequestStarted).Should(Succeed())
			Eventually(verifyJobRequestStartedEventEmitted).Should(Succeed())

			By("Job fails to perform rake task")
			verifyJobFailed := func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "jobs", "govuk-replatform-test-app",
					"-o", "jsonpath={.status.failed}",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("1"), "Job not failed")
			}
			verifyJobRequestFailedEventEmitted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "events", "--field-selector",
					"involvedObject.kind=JobRequest,reason=Failed",
					"-o", "json",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				eventList := &eventsv1.EventList{}
				err = json.Unmarshal([]byte(output), &eventList)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Failed"))
			}

			Eventually(verifyJobFailed).Should(Succeed())
			Eventually(verifyJobRequestFailedEventEmitted).Should(Succeed())
		})

		It("should not create a job when JobRequest is rejected", func() {
			By("creating a govuk-replatform-test-app Deployment for the JobRequest to run a rake task from")
			deploymentFixture, err := utils.RetrieveFixtureFilePath(govukReplatformTestAppDeployment)
			Expect(err).NotTo(HaveOccurred(), "Failed to retrieve deployment fixture filepath")

			cmd := exec.Command("kubectl", "apply", "-f", deploymentFixture, "-n", appNamespace)

			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create govuk-replatform-test-app deployment")

			By("waiting for the govuk-replatform-test-app deployment to become available.")
			verifyDeploymentInAvailableState := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "deployments", "govuk-replatform-test-app",
					"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}", "-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("True"), "govuk-replatform-test-app deployment not ready")
			}
			Eventually(verifyDeploymentInAvailableState).Should(Succeed())

			By("creating a JobRequest")
			jobRequestFixture, err := utils.RetrieveFixtureFilePath(jobRequestForSuccessfulJob)
			Expect(err).NotTo(HaveOccurred(), "Failed to retrieve current working directory")

			cmd = exec.Command("kubectl", "apply", "-f", jobRequestFixture, "-n", appNamespace)

			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create govuk-replatform-test-app jobRequest")

			verifyJobRequestInPendingState := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "jobrequests.platform.publishing.service.gov.uk", "govuk-replatform-test-app",
					"-o", "jsonpath={.status.state}",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Pending"), "JobRequest in wrong status")
			}
			verifyJobRequestPendingEventEmitted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "events", "--field-selector",
					"involvedObject.kind=JobRequest,reason=Pending",
					"-o", "json",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				eventList := &eventsv1.EventList{}
				err = json.Unmarshal([]byte(output), &eventList)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Pending"))
			}
			Eventually(verifyJobRequestInPendingState).Should(Succeed())
			Eventually(verifyJobRequestPendingEventEmitted).Should(Succeed())

			By("creating a JobRequestReview to reject the JobRequest")
			jobRequestReviewRejectedFixture, err := utils.RetrieveFixtureFilePath(jobRequestReviewRejected)
			Expect(err).NotTo(HaveOccurred(), "Failed to retrieve current working directory")

			cmd = exec.Command("kubectl", "apply", "-f", jobRequestReviewRejectedFixture, "-n", appNamespace)

			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create govuk-replatform-test-app jobRequestReviewApproved")

			verifyJobRequestReviewInRejectedState := func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "jobrequestreviews.platform.publishing.service.gov.uk", "govuk-replatform-test-app",
					"-o", "jsonpath={.status.state}",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Rejected"), "JobRequestReview in wrong status")
			}

			verifyJobRequestReviewRejectedEventEmitted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "events", "--field-selector",
					"involvedObject.kind=JobRequestReview,reason=Rejected",
					"-o", "json",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				eventList := &eventsv1.EventList{}
				err = json.Unmarshal([]byte(output), &eventList)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Rejected"))
			}

			verifyJobRequestRejectedEventEmitted := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "events", "--field-selector",
					"involvedObject.kind=JobRequest,reason=Rejected",
					"-o", "json",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())

				eventList := &eventsv1.EventList{}
				err = json.Unmarshal([]byte(output), &eventList)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(eventList.Items).To(HaveLen(1))
				g.Expect(eventList.Items[0].Reason).To(Equal("Rejected"))
			}
			Eventually(verifyJobRequestReviewInRejectedState).Should(Succeed())
			Eventually(verifyJobRequestReviewRejectedEventEmitted).Should(Succeed())
			Eventually(verifyJobRequestRejectedEventEmitted).Should(Succeed())

			By("JobRequest is in Rejected state")
			verifyJobRequestStarted := func(g Gomega) {
				jobRequestStateJSON := `{"reviewName":"govuk-replatform-test-app","state":"Rejected"}`
				cmd = exec.Command("kubectl", "get", "jobrequests.platform.publishing.service.gov.uk", "govuk-replatform-test-app",
					"-o", "jsonpath={.status}",
					"-n", appNamespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).Should(MatchJSON(jobRequestStateJSON), "JobRequest in wrong status")
			}

			Eventually(verifyJobRequestStarted).Should(Succeed())

			By("Job not created")
			verifyJobNotStarted := func(g Gomega) {
				cmd = exec.Command("kubectl", "get", "jobs", "govuk-replatform-test-app", "-n", appNamespace)
				_, err := utils.Run(cmd)
				g.Expect(err).To(HaveOccurred())
			}

			Eventually(verifyJobNotStarted).Should(Succeed())
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks

	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	By("creating temporary file to store the token request")
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		By("executing kubectl command to create the token")
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			controllerNamespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		By("parsing the JSON output to extract the token")
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() (string, error) {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", controllerNamespace)
	return utils.Run(cmd)
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
