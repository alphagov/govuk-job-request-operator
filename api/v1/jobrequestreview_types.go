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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JobRequestReviewSpec defines the desired state of JobRequestReview
type JobRequestReviewSpec struct {
	// Name of the JobRequest resource being reviewed.
	JobRequestName string `json:"jobRequestName"`
	// +kubebuilder:validation:Enum=Approved;Rejected
	Decision string `json:"decision"`
	// A description of the review decision.
	// +optional
	Description string `json:"description,omitempty"`
}

// JobRequestReviewStatus defines the observed state of JobRequestReview.
type JobRequestReviewStatus struct {
	// +kubebuilder:validation:Enum=Approved;Rejected;JobRequestMalformed;JobRequestNotFound
	State JobRequestReviewState `json:"state,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=jrr
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Job Request",type=string,JSONPath=`.spec.jobRequestName`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:selectablefield:JSONPath=".spec.jobRequestName"

// JobRequestReview represents a decision to run a requested job in the cluster
type JobRequestReview struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of JobRequestReview
	// +required
	Spec JobRequestReviewSpec `json:"spec"`

	// status defines the observed state of JobRequestReview
	// +optional
	Status JobRequestReviewStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// JobRequestReviewList contains a list of JobRequestReview
type JobRequestReviewList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []JobRequestReview `json:"items"`
}

type JobRequestReviewState string

const (
	JobRequestReviewApproved  JobRequestReviewState = "Approved"
	JobRequestReviewRejected  JobRequestReviewState = "Rejected"
	JobRequestReviewMalformed JobRequestReviewState = "JobRequestMalformed"
	JobRequestReviewNotFound  JobRequestReviewState = "JobRequestNotFound"
)
