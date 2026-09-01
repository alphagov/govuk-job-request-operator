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
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/alphagov/govuk-job-request-operator/test/utils"
)

type ClusterUsers []*ClusterUser

type ClusterUser struct {
	Name                string
	ARN                 string
	KubectlUserName     string
	Base64EncodedCSR    string
	KeyFilePath         string
	CSRFilePath         string
	CSRManifestPath     string
	CertificateFilePath string
}

var (
	// managerImage is the manager image to be built and loaded for testing.
	managerImage = "ghcr.io/alphagov/govuk/govuk-job-request-operator:v0.0.1"
	// kindCluster is the name of the Kind cluster to be used for testing.
	kindCluster = utils.DefaultKindCluster
	// shouldCleanupCertManager tracks whether CertManager was installed by this suite.
	shouldCleanupCertManager = false
	// govukReplatformTestAppImage is the image used for testing
	govukReplatformTestAppImage = "ghcr.io/alphagov/govuk/govuk-replatform-test-app:v48"
	// JobRequesterUser is the user to use for creating JobRequest resources
	JobRequesterUser = &ClusterUser{
		Name: "job-requester",
		ARN:  "arn:aws:sts::123456789012:assumed-role/job.req-developer/e2e",
	}
	// JobReviewerUser is the user to use for creating JobRequestReview resources
	JobReviewerUser = &ClusterUser{
		Name: "job-reviewer",
		ARN:  "arn:aws:sts::123456789012:assumed-role/job.rev-developer/e2e",
	}
	// KubernetesUsers will have kubernetes users provisioned into the cluster. Only Name and ARN need to be specified
	KubernetesUsers = &ClusterUsers{
		JobRequesterUser,
		JobReviewerUser,
	}
)

// To skip CertManager installation, set: CERT_MANAGER_INSTALL_SKIP=true
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting govuk-job-request-operator e2e test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	By("creating a Kind cluster for e2e tests")
	cmd := exec.Command("kind", "create", "cluster", "--name", kindCluster)
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to create Kind cluster")

	setupUsers()

	By("building the manager image")
	cmd = exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", managerImage))
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager image")

	By("loading the manager image on Kind")
	err = utils.LoadImageToKindClusterWithName(managerImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager image into Kind")

	By("loading the govuk-replatform-test-app image on Kind")
	// This command is a workaround to kind not supporting the docker-desktop containerd image store fully
	// See https://github.com/kubernetes-sigs/kind/issues/3795, once this is resolved we should be able
	// to just docker pull the image and call utils.LoadImageToKindClusterWithName on it
	cmd = exec.Command("docker", "exec", fmt.Sprintf("%s-control-plane", kindCluster), "ctr", "--namespace=k8s.io", "images", "pull", govukReplatformTestAppImage)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to load the govuk-replatform-test-app image into Kind")

	setupCertManager()
})

var _ = AfterSuite(func() {
	By("deleting the kubernetes users from kubeconfig")
	deleteKubernetesUsersFromKubeconfig(context.Background())

	By("deleting the Kind cluster")
	cmd := exec.Command("kind", "delete", "cluster", "--name", kindCluster)
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to delete Kind cluster")
})

// setupCertManager installs CertManager if needed for webhook tests.
// Skips installation if CERT_MANAGER_INSTALL_SKIP=true or if already present.
func setupCertManager() {
	if os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true" {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping CertManager installation (CERT_MANAGER_INSTALL_SKIP=true)\n")
		return
	}

	By("checking if CertManager is already installed")
	if utils.IsCertManagerCRDsInstalled() {
		_, _ = fmt.Fprintf(GinkgoWriter, "CertManager is already installed. Skipping installation.\n")
		return
	}

	// Mark for cleanup before installation to handle interruptions and partial installs.
	shouldCleanupCertManager = true

	By("installing CertManager")
	Expect(utils.InstallCertManager()).To(Succeed(), "Failed to install CertManager")
}

func applyKubernetesManifest(ctx context.Context, manifestPath string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", manifestPath)
	_, err := utils.Run(cmd)
	if err != nil {
		return err
	}

	return nil
}

func setupUsers() {
	ctx := context.Background()

	By("Setting up users in the cluster")
	tempDir, err := os.MkdirTemp("", "govuk-job-request-operator-e2e-*")
	Expect(err).NotTo(HaveOccurred(), "Couldn't create tempdir for setting up users")

	for _, user := range *KubernetesUsers {
		err = os.Mkdir(filepath.Join(tempDir, user.Name), 0700)
		Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to create tempdir for user %s", user.Name))
	}

	By("Creating the users in the cluster")
	for _, user := range *KubernetesUsers {
		user.KeyFilePath = filepath.Join(tempDir, user.Name, "e2e-cert.key")
		user.CSRFilePath = filepath.Join(tempDir, user.Name, "e2e-cert.csr")
		user.CSRManifestPath = filepath.Join(tempDir, user.Name, "e2e-csr-manifest.yaml")
		user.CertificateFilePath = filepath.Join(tempDir, user.Name, "e2e-signed.crt")
		user.KubectlUserName = fmt.Sprintf("govuk-job-request-operator-e2e-%s", user.Name)

		By(fmt.Sprintf("Generating the certificate for user %s", user.Name))
		cmd := exec.CommandContext(ctx, "openssl", "genrsa", "-out", user.KeyFilePath)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to generate user certificate")

		By(fmt.Sprintf("Generating the certificate signing request for user %s", user.Name))
		// The /'s in the ARNs must have an escape character in the final arg sent to the openssl command to be a valid CN
		cmd = exec.CommandContext(
			ctx, "openssl", "req", "-new",
			"-key", user.KeyFilePath, "-out", user.CSRFilePath,
			"-subj", fmt.Sprintf("/CN=%s", strings.ReplaceAll(user.ARN, "/", "\\/")),
		)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to generate user certificate signing request")

		By(fmt.Sprintf("Generating the CSR kubernetes manifest for user %s", user.Name))
		csr, err := os.ReadFile(user.CSRFilePath)
		Expect(err).NotTo(HaveOccurred(), "Failed reading CSR file")
		user.Base64EncodedCSR = base64.StdEncoding.EncodeToString(csr)

		By("Applying the CSR request manifest")
		renderTemplate("user_setup/certificate_signing_request.template.yaml", user.CSRManifestPath, user)

		By("Applying the CSR to the cluster")
		err = applyKubernetesManifest(ctx, user.CSRManifestPath)
		Expect(err).NotTo(HaveOccurred(), "Failed applying CSR Manigest")

		By("Approving the CSR request in kubernetes")
		cmd = exec.CommandContext(ctx, "kubectl", "certificate", "approve", user.Name)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to approve the CSR")

		By("Waiting for the certificate to be issued")
		cmd = exec.CommandContext(ctx, "kubectl", "wait", "csr", user.Name, "--for", "jsonpath={.status.certificate}", "--timeout", "1m")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "CSR failed to issue after being approved")

		By("Saving the signed certificate")
		cmd = exec.CommandContext(ctx, "kubectl", "get", "csr", user.Name, "-o", "jsonpath={.status.certificate}")
		base64EncodedSignedCertificate, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to fetch base64 encoded signed certificate")

		data, err := base64.StdEncoding.DecodeString(base64EncodedSignedCertificate)
		Expect(err).NotTo(HaveOccurred(), "Failed to base64 decode the signed certificate")

		err = os.WriteFile(user.CertificateFilePath, data, 0600)
		Expect(err).NotTo(HaveOccurred(), "Failed to write signed certificate to disk")

		cmd = exec.CommandContext(
			ctx,
			"kubectl", "config", "set-credentials", user.KubectlUserName,
			fmt.Sprintf("--client-key=%s", user.KeyFilePath),
			fmt.Sprintf("--client-certificate=%s", user.CertificateFilePath),
			"--embed-certs=true",
		)
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred())
	}

	By("Applying the cluster role bindings")
	roleBindingManifestFilePath := filepath.Join(tempDir, "role_bindings.yaml")
	renderTemplate("user_setup/role_binding.template.yaml", roleBindingManifestFilePath, *KubernetesUsers)
	err = applyKubernetesManifest(ctx, roleBindingManifestFilePath)
	Expect(err).NotTo(HaveOccurred(), "Failed to apply role binding manifest")
}

func SwitchToKubernetesAdminUser(ctx context.Context) {
	By("switching to the kubernetes-admin user")
	switchToUser(ctx, "kind-govuk-job-request-operator-test-e2e")
}

func SwitchToKubernetesUser(ctx context.Context, clusterUser *ClusterUser) {
	By(fmt.Sprintf("switching to the %s user", clusterUser.Name))
	switchToUser(ctx, clusterUser.KubectlUserName)
}

func switchToUser(ctx context.Context, kubectlUserName string) {
	cmd := exec.CommandContext(ctx, "kubectl", "config", "set-context", "--current", "--user", kubectlUserName)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())
}

func renderTemplate(templatePath, outputPath string, templateData any) {
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

func deleteKubernetesUsersFromKubeconfig(ctx context.Context) {
	for _, user := range *KubernetesUsers {
		cmd := exec.CommandContext(ctx, "kubectl", "config", "delete-user", user.KubectlUserName)
		_, err := utils.Run(cmd)
		// This is only called in shutdown, and we don't want to fail the suite shutdown if this errors, so don't Expect success
		if err != nil {
			fmt.Sprintf("Failed to delete user %s from kubectl config", user.KubectlUserName)
		}
	}
}
