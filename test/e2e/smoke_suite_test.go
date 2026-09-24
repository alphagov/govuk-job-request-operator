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

var (
	// JobRequesterUser is the user to use for creating JobRequest resources
	JobRequesterUser = &utils.ClusterUser{
		Name: "job-requester",
		ARN:  "arn:aws:sts::123456789012:assumed-role/job.req-developer/e2e",
	}
	// JobReviewerUser is the user to use for creating JobRequestReview resources
	JobReviewerUser = &utils.ClusterUser{
		Name: "job-reviewer",
		ARN:  "arn:aws:sts::123456789012:assumed-role/job.rev-developer/e2e",
	}
	jobRequestImpersonateUser = JobRequesterUser.ARN
	jobReviewImpersonateUser  = JobReviewerUser.ARN
)

const (
	// smokeTestServiceAccountName created for smoke test job
	smokeTestServiceAccountName = "kubectl-impersonator"
	// cluster name
	clusterName = "in-cluster"
	// appNamespace is the namespace used by the in-cluster smoke tests.
	appNamespace = "default"
	// controllerNamespace matches the default in-cluster deployment namespace.
	controllerNamespace = "default"
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
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to configure in-cluster kubectl cluster")

	By("Setting service account credentials for smoke tests")
	cmd = exec.CommandContext(ctx, "kubectl", "config", "set-credentials", serviceAccountName,
		"--token", readServiceAccountToken(),
	)
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to configure in-cluster kubectl credentials")

	By("Setting up kubectl in-cluster context for smoke tests")
	cmd = exec.CommandContext(ctx, "kubectl", "config", "set-context", clusterName,
		"--cluster", clusterName,
		"--user", serviceAccountName,
	)
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to configure in-cluster kubectl context")

	By("Use the in-cluster context for smoke tests")
	cmd = exec.CommandContext(ctx, "kubectl", "config", "use-context", clusterName)
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to select in-cluster kubectl context")
})
