//go:build smoke
// +build smoke

package e2e

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/alphagov/govuk-job-request-operator/test/utils"
)

const (
	smokeTestServiceAccountName = "smoke-test-runner-sa"
	clusterName                 = "cluster"
	// namespace used by the smoke tests to create JobRequest and JobRequestReview resources
	appNamespace = "job-request-operator-smoke-test"
)

func TestSmokeE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "smoke e2e suite")
}

func readServiceAccountToken() string {
	token, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to read Kubernetes service account token")
	return strings.TrimSpace(string(token))
}

var _ = BeforeSuite(func(ctx context.Context) {
	By("Retrieving cluster information for smoke tests")
	cmd := exec.CommandContext(ctx, "kubectl", "config", "set-cluster", clusterName,
		"--server=https://"+os.Getenv("KUBERNETES_SERVICE_HOST")+":"+os.Getenv("KUBERNETES_SERVICE_PORT"),
		"--certificate-authority=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
	)
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to configure cluster information for kubectl")

	By("Setting service account credentials for smoke tests")
	cmd = exec.CommandContext(ctx, "kubectl", "config", "set-credentials", smokeTestServiceAccountName,
		"--token", readServiceAccountToken(),
	)
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to configure in-cluster kubectl credentials")

	By("Setting up kubectl in-cluster context for smoke tests")
	cmd = exec.CommandContext(ctx, "kubectl", "config", "set-context", clusterName,
		"--cluster", clusterName,
		"--user", smokeTestServiceAccountName,
	)
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to configure kubectl context")

	By("Use the in-cluster context for smoke tests")
	cmd = exec.CommandContext(ctx, "kubectl", "config", "use-context", clusterName)
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to select in-cluster kubectl context")
})
