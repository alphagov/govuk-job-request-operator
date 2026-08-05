package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("JobRequestReview", Ordered, func() {
	Describe("GetReviewedBy", func() {
		It("Returns the annotation value and a nil error if the annotation is set", func() {
			jobRequestReview := &JobRequestReview{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"platform.publishing.service.gov.uk/reviewed-by": "joe.blogs",
					},
				},
			}

			reviewedBy, err := jobRequestReview.GetReviewedBy()
			Expect(reviewedBy).To(Equal("joe.blogs"))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Returns an empty string and an error if the annotation is not set", func() {
			jobRequestReview := &JobRequestReview{}

			reviewedBy, err := jobRequestReview.GetReviewedBy()
			Expect(reviewedBy).To(Equal(""))
			Expect(err).To(HaveOccurred())
		})
	})
})
