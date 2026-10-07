//go:build smoke
// +build smoke

package e2e

import (
	"context"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

const (
	smokeTestServiceAccountName = "job-request-operator-smoke-test-runner"
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
	homeDir, err := os.UserHomeDir()
	server := "https://" + os.Getenv("KUBERNETES_SERVICE_HOST") + ":" + os.Getenv("KUBERNETES_SERVICE_PORT")
	certificateAuthority := "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"

	config := api.NewConfig()

	config.Clusters[clusterName] = &api.Cluster{
		Server:                server,
		InsecureSkipTLSVerify: false,
		CertificateAuthority:  certificateAuthority,
	}

	By("Setting service account credentials for smoke tests")
	config.AuthInfos[smokeTestServiceAccountName] = &api.AuthInfo{
		Token: readServiceAccountToken(),
	}

	By("Setting up kubectl cluster context for smoke tests")
	config.Contexts[clusterName] = &api.Context{
		Cluster:   clusterName,
		AuthInfo:  smokeTestServiceAccountName,
		Namespace: appNamespace,
	}

	By("Use the cluster context for smoke tests")
	config.CurrentContext = clusterName

	By("Write kubeconfig for smoke tests")
	err = clientcmd.WriteToFile(*config, homeDir+"/.kube/config")
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to write kubeconfig for smoke tests")

})

var _ = ReportAfterSuite("Push smoke test metrics to Prometheus Pushgateway", func(report Report) {
	By("Setting up Prometheus Pushgateway")
	reg := prometheus.NewRegistry()
	smokeTestSuccess := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "job_request_operator_smoke_test_success",
		Help: "Status of the job request operator smoke test execution",
	})
	reg.MustRegister(smokeTestSuccess)

	prometheusPushGatewayUrl := os.Getenv("PROMETHEUS_PUSHGATEWAY_URL")

	if report.SuiteSucceeded {
		smokeTestSuccess.Set(1)
	} else {
		smokeTestSuccess.Set(0)
	}

	By("Pushing job_request_operator_smoke_test_success_metric to Prometheus Pushgateway")
	err := push.New(prometheusPushGatewayUrl, "job_request_operator_smoke_test_metrics").Gatherer(reg).Push()
	Expect(err).NotTo(HaveOccurred(), "Error pushing smoke test metrics to Prometheus Pushgateway")
})
