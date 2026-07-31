package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const jobRequestName string = "test-job-request"
const jobRequestReviewName string = "test-job-request-review"

func jobRequestWithState(state JobRequestState, reviewedBy string) *JobRequest {
	return &JobRequest{
		ObjectMeta: metav1.ObjectMeta{Name: jobRequestName},
		Status: JobRequestStatus{
			State:      state,
			ReviewName: reviewedBy,
		},
	}
}

func jobRequestReview(jobRequestName string, decision JobRequestReviewState) *JobRequestReview {
	return &JobRequestReview{
		ObjectMeta: metav1.ObjectMeta{Name: jobRequestReviewName},
		Spec: JobRequestReviewSpec{
			JobRequestName: jobRequestName,
			Decision:       string(decision),
		},
	}
}

var _ = Describe("JobRequest", Ordered, func() {
	Describe("WasReviewedBy", func() {
		It("returns false if the JobRequest has no state yet", func() {
			jobRequest := &JobRequest{}
			jobRequestReview := jobRequestReview(jobRequestName, JobRequestReviewApproved)

			Expect(jobRequest.WasReviewedBy(jobRequestReview)).To(BeFalse())
		})

		It("returns false if the JobRequest is still Pending", func() {
			jobRequest := jobRequestWithState(JobRequestPending, "")
			jobRequestReview := jobRequestReview(jobRequestName, JobRequestReviewApproved)

			Expect(jobRequest.WasReviewedBy(jobRequestReview)).To(BeFalse())
		})

		It("returns false if the JobRequest was reviewed by a different JobRequestReview", func() {
			jobRequest := jobRequestWithState(JobRequestPending, "different-test-review")
			jobRequestReview := jobRequestReview(jobRequestName, JobRequestReviewApproved)

			Expect(jobRequest.WasReviewedBy(jobRequestReview)).To(BeFalse())
		})

		It("returns true if the JobRequest was reviewed by the passed in JobRequestReview", func() {
			jobRequest := jobRequestWithState(JobRequestPending, jobRequestReviewName)
			jobRequestReview := jobRequestReview(jobRequestName, JobRequestReviewApproved)

			Expect(jobRequest.WasReviewedBy(jobRequestReview)).To(BeTrue())
		})
	})

	Describe("HasBeenRejected", func() {
		It("returns false if the JobRequest has no state yet", func() {
			jobRequest := &JobRequest{}

			Expect(jobRequest.HasBeenRejected()).To(BeFalse())
		})

		It("returns true if the JobRequest has been rejected", func() {
			jobRequest := jobRequestWithState(JobRequestRejected, jobRequestReviewName)

			Expect(jobRequest.HasBeenRejected()).To(BeTrue())
		})

		DescribeTable("returns false for states",
			func(state JobRequestState) {
				jobRequest := jobRequestWithState(state, jobRequestReviewName)
				Expect(jobRequest.HasBeenRejected()).To(BeFalse())
			},
			Entry("when the JobRequest is in state Pending", JobRequestPending),
			Entry("when the JobRequest is in state Approved", JobRequestApproved),
			Entry("when the JobRequest is in state Started", JobRequestStarted),
			Entry("when the JobRequest is in state Complete", JobRequestComplete),
			Entry("when the JobRequest is in state Failed", JobRequestFailed),
			Entry("when the JobRequest is in state Malformed", JobRequestMalformed),
		)
	})

	Describe("HasBeenApproved", func() {
		It("returns false if the JobRequest has no state yet", func() {
			jobRequest := &JobRequest{}

			Expect(jobRequest.HasBeenApproved()).To(BeFalse())
		})

		DescribeTable("returns false for states",
			func(state JobRequestState) {
				jobRequest := jobRequestWithState(state, jobRequestReviewName)
				Expect(jobRequest.HasBeenApproved()).To(BeFalse())
			},
			Entry("when the JobRequest is in state Pending", JobRequestPending),
			Entry("when the JobRequest is in state Rejected", JobRequestRejected),
			Entry("when the JobRequest is in state Malformed", JobRequestMalformed),
		)

		DescribeTable("returns true for states",
			func(state JobRequestState) {
				jobRequest := jobRequestWithState(state, jobRequestReviewName)
				Expect(jobRequest.HasBeenApproved()).To(BeTrue())
			},
			Entry("when the JobRequest is in state Approved", JobRequestApproved),
			Entry("when the JobRequest is in state Started", JobRequestStarted),
			Entry("when the JobRequest is in state Complete", JobRequestComplete),
			Entry("when the JobRequest is in state Failed", JobRequestFailed),
		)
	})

	Describe("HasBeenReviewed", func() {
		It("returns false if the JobRequest has no state yet", func() {
			jobRequest := &JobRequest{}

			Expect(jobRequest.HasBeenReviewed()).To(BeFalse())
		})

		DescribeTable("returns false for states",
			func(state JobRequestState) {
				jobRequest := jobRequestWithState(state, jobRequestReviewName)
				Expect(jobRequest.HasBeenReviewed()).To(BeFalse())
			},
			Entry("when the JobRequest is in state Pending", JobRequestPending),
			Entry("when the JobRequest is in state Malformed", JobRequestMalformed),
		)

		DescribeTable("returns true for states",
			func(state JobRequestState) {
				jobRequest := jobRequestWithState(state, jobRequestReviewName)
				Expect(jobRequest.HasBeenReviewed()).To(BeTrue())
			},
			Entry("when the JobRequest is in state Approved", JobRequestApproved),
			Entry("when the JobRequest is in state Rejected", JobRequestRejected),
			Entry("when the JobRequest is in state Started", JobRequestStarted),
			Entry("when the JobRequest is in state Complete", JobRequestComplete),
			Entry("when the JobRequest is in state Failed", JobRequestFailed),
		)
	})
})
