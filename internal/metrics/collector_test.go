package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
	log "sigs.k8s.io/controller-runtime/pkg/log"
)

type MockClient struct {
	client.Client
	ListFunc func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}

func (m *MockClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, list, opts...)
	}
	return nil
}

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MetricCollector Suite")
}

var _ = Describe("MetricCollector", func() {
	var (
		collector  *MetricCollector
		mockClient *MockClient
		ctx        context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockClient = &MockClient{}
		collector = &MetricCollector{
			CacheClient:   mockClient,
			Interval:      1 * time.Second,
			Log:           log.Log,
			CustomMetrics: InitCollectorCustomMetrics(),
		}
	})

	Describe("collect", func() {
		Context("when listing resources succeeds", func() {
			It("should correctly tally JobRequest and JobRequestReview states", func() {
				mockClient.ListFunc = func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
					switch l := list.(type) {
					case *platformv1.JobRequestList:
						l.Items = []platformv1.JobRequest{
							{Status: platformv1.JobRequestStatus{State: platformv1.JobRequestPending}},
							{Status: platformv1.JobRequestStatus{State: platformv1.JobRequestPending}},
							{Status: platformv1.JobRequestStatus{State: platformv1.JobRequestApproved}},
							{Status: platformv1.JobRequestStatus{State: platformv1.JobRequestRejected}},
						}
						return nil
					case *platformv1.JobRequestReviewList:
						l.Items = []platformv1.JobRequestReview{
							{Status: platformv1.JobRequestReviewStatus{State: platformv1.JobRequestReviewApproved}},
						}
						return nil
					default:
						return errors.New("unexpected list type")
					}
				}

				collector.collect(ctx)

				// Verify Request Metrics
				Expect(testutil.ToFloat64(collector.CustomMetrics.RequestCurrentPendingTotal)).To(Equal(float64(2)))
				Expect(testutil.ToFloat64(collector.CustomMetrics.RequestCurrentApprovedTotal)).To(Equal(float64(1)))
				Expect(testutil.ToFloat64(collector.CustomMetrics.RequestCurrentRejectedTotal)).To(Equal(float64(1)))

				// Verify Review Metrics
				Expect(testutil.ToFloat64(collector.CustomMetrics.ReviewCurrentApprovedTotal)).To(Equal(float64(1)))
			})

			It("should correctly tally JobRequest and JobRequestReview states when both are 0", func() {
				mockClient.ListFunc = func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
					switch l := list.(type) {
					case *platformv1.JobRequestList:
						l.Items = []platformv1.JobRequest{}
						return nil
					case *platformv1.JobRequestReviewList:
						l.Items = []platformv1.JobRequestReview{}
						return nil
					default:
						return errors.New("unexpected list type")
					}
				}

				collector.collect(ctx)

				// Verify Request Metrics
				Expect(testutil.ToFloat64(collector.CustomMetrics.RequestCurrentPendingTotal)).To(Equal(float64(0)))
				Expect(testutil.ToFloat64(collector.CustomMetrics.RequestCurrentApprovedTotal)).To(Equal(float64(0)))
				Expect(testutil.ToFloat64(collector.CustomMetrics.RequestCurrentRejectedTotal)).To(Equal(float64(0)))

				// Verify Review Metrics
				Expect(testutil.ToFloat64(collector.CustomMetrics.ReviewCurrentApprovedTotal)).To(Equal(float64(0)))
			})
		})

		Context("when listing resources fails", func() {
			It("should log an error and not panic", func() {
				mockClient.ListFunc = func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
					return errors.New("api error")
				}

				// We just want to ensure it doesn't panic
				Expect(func() { collector.collect(ctx) }).NotTo(Panic())
			})
		})
	})
})
