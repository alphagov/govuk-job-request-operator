//go:build e2e
// +build e2e

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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"text/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/alphagov/govuk-job-request-operator/test/utils"
)

var (
	// shouldCleanupCertManager tracks whether CertManager was installed by this suite.
	shouldCleanupCertManager = false
)

const (
	// managerImage is the manager image to be built and loaded for testing.
	managerImage = "ghcr.io/alphagov/govuk/govuk-job-request-operator:v0.0.1"
	// kindCluster is the name of the Kind cluster to be used for testing.
	kindCluster = utils.DefaultKindCluster
	// govukReplatformTestAppImage is the image used for testing
	govukReplatformTestAppImage = "ghcr.io/alphagov/govuk/govuk-replatform-test-app:v48"
	// namespace where resources are deployed in
	appNamespace = "apps"
	// namespace where the operator is deployed in
	controllerNamespace = "govuk-job-request-operator-system"
)

// To skip CertManager installation, set: CERT_MANAGER_INSTALL_SKIP=true
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting govuk-job-request-operator e2e test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func(ctx context.Context) {
	By("creating a Kind cluster for e2e tests")
	cmd := exec.CommandContext(ctx, "kind", "create", "cluster", "--name", kindCluster)
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to create Kind cluster")

	By("building the manager image")
	cmd = exec.CommandContext(ctx, "make", "docker-build", fmt.Sprintf("IMG=%s", managerImage))
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager image")

	By("loading the manager image on Kind")
	err = utils.LoadImageToKindClusterWithName(ctx, managerImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager image into Kind")

	By("loading the govuk-replatform-test-app image on Kind")
	// This command is a workaround to kind not supporting the docker-desktop containerd image store fully
	// See https://github.com/kubernetes-sigs/kind/issues/3795, once this is resolved we should be able
	// to just docker pull the image and call utils.LoadImageToKindClusterWithName on it
	cmd = exec.CommandContext(ctx,
		"docker", "exec", fmt.Sprintf("%s-control-plane", kindCluster),
		"ctr", "--namespace=k8s.io", "images", "pull", govukReplatformTestAppImage,
	)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to load the govuk-replatform-test-app image into Kind")

	setupCertManager(ctx)

	By("creating manager namespace")
	cmd = exec.CommandContext(ctx, "kubectl", "create", "ns", controllerNamespace)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

	By("labeling the namespace to enforce the restricted security policy")
	cmd = exec.CommandContext(ctx, "kubectl", "label", "--overwrite", "ns", controllerNamespace,
		"pod-security.kubernetes.io/enforce=restricted")
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

	By("installing CRDs")
	cmd = exec.CommandContext(ctx, "make", "install")
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

	By("waiting for CRDs to become available")
	Eventually(ctx, func(g Gomega) {
		cmd := exec.CommandContext(ctx, "kubectl", "get", "--raw", "/apis/platform.publishing.service.gov.uk/v1")
		_, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred())
	}).Should(Succeed())

	By("deploying the controller-manager")
	cmd = exec.CommandContext(ctx, "make", "deploy", fmt.Sprintf("IMG=%s", managerImage))
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")

	By("creating apps namespace")
	cmd = exec.CommandContext(ctx, "kubectl", "create", "ns", appNamespace)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create apps namespace")

	By("labeling the apps namespace to enforce the restricted security policy")
	cmd = exec.CommandContext(ctx, "kubectl", "label", "--overwrite", "ns", appNamespace,
		"pod-security.kubernetes.io/enforce=restricted")
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to label apps namespace with restricted policy")

	users := []string{
		jobRequestImpersonateUser,
		jobReviewImpersonateUser,
	}

	setupUsers(ctx, users)
})

var _ = AfterSuite(func(ctx context.Context) {
	By("cleaning up the curl pod for metrics")
	cmd := exec.CommandContext(ctx, "kubectl", "delete", "pod", "curl-metrics", "-n", controllerNamespace)
	_, _ = utils.Run(cmd)

	By("removing apps namespace")
	cmd = exec.CommandContext(ctx, "kubectl", "delete", "ns", appNamespace)
	_, _ = utils.Run(cmd)

	By("undeploying the controller-manager")
	cmd = exec.CommandContext(ctx, "make", "undeploy")
	_, _ = utils.Run(cmd)

	By("uninstalling CRDs")
	cmd = exec.CommandContext(ctx, "make", "uninstall")
	_, _ = utils.Run(cmd)

	By("removing manager namespace")
	cmd = exec.CommandContext(ctx, "kubectl", "delete", "ns", controllerNamespace)
	_, _ = utils.Run(cmd)

	By("deleting the Kind cluster")
	cmd = exec.CommandContext(ctx, "kind", "delete", "cluster", "--name", kindCluster)
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to delete Kind cluster")
})

// setupCertManager installs CertManager if needed for webhook tests.
// Skips installation if CERT_MANAGER_INSTALL_SKIP=true or if already present.
func setupCertManager(ctx context.Context) {
	if os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true" {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping CertManager installation (CERT_MANAGER_INSTALL_SKIP=true)\n")
		return
	}

	By("checking if CertManager is already installed")
	if utils.IsCertManagerCRDsInstalled(ctx) {
		_, _ = fmt.Fprintf(GinkgoWriter, "CertManager is already installed. Skipping installation.\n")
		return
	}

	// Mark for cleanup before installation to handle interruptions and partial installs.
	shouldCleanupCertManager = true

	By("installing CertManager")
	Expect(utils.InstallCertManager(ctx)).To(Succeed(), "Failed to install CertManager")
}

func applyKubernetesManifest(ctx context.Context, manifestPath string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", manifestPath)
	_, err := utils.Run(cmd)
	if err != nil {
		return err
	}

	return nil
}

func setupUsers(ctx context.Context, users []string) {
	By("Setup Role to create and retrieve JobRequests and JobRequestReviews")
	roleFilePath, err := utils.RetrieveFixtureFilePath("user_setup/job_request_create_get_role.yaml")
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to retrieve fixture with path %s", roleFilePath))
	err = applyKubernetesManifest(ctx, roleFilePath)

	By("Setting up users in the cluster")
	tempDir, err := os.MkdirTemp("", "govuk-job-request-operator-e2e-*")
	Expect(err).NotTo(HaveOccurred(), "Couldn't create tempdir for setting up users")

	By("Applying the role bindings")
	roleBindingManifestFilePath := filepath.Join(tempDir, "role_bindings.yaml")
	renderTemplate("user_setup/role_binding.template.yaml", roleBindingManifestFilePath, users)
	err = applyKubernetesManifest(ctx, roleBindingManifestFilePath)
	Expect(err).NotTo(HaveOccurred(), "Failed to apply role binding manifest")
}

func renderTemplate(templatePath, outputPath string, templateData []string) {
	templatePath, err := utils.RetrieveFixtureFilePath(templatePath)
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to retrieve fixture with path %s", templatePath))

	parsedTemplate, err := template.ParseFiles(templatePath)
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to load and parse request template with path %s", templatePath))

	fileWriter, err := os.Create(outputPath)
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to create output path %s", outputPath))
	defer fileWriter.Close()

	parsedTemplate.Execute(fileWriter, templateData)
	Expect(err).NotTo(HaveOccurred(), "Failed executing template")
}
