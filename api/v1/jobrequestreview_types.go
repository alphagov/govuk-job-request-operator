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
