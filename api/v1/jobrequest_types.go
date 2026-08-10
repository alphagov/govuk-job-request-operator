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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type JobRequestPodSpecFrom struct {
	// +kubebuilder:validation:Enum=apps/v1;
	Group string `json:"group"`
	// +kubebuilder:validation:Enum=Deployment;
	Kind string `json:"kind"`
	// Resource name which contains the pod spec to use for the job.
	Name string `json:"name"`
}

type JobRequestContainerFrom struct {
	// Where to get the pod spec for the job from.
	PodSpecFrom JobRequestPodSpecFrom `json:"podSpecFrom"`
	// The name of the container in the pod spec to use for the job.
	ContainerName string `json:"containerName"`
}

// JobRequestSpec defines the desired state of JobRequest
type JobRequestSpec struct {
	// Where to get the container and pod spec for the job from.
	ContainerFrom JobRequestContainerFrom `json:"containerFrom"`
	// Command to run in the job.
	Command string `json:"command"`
	// Arguments to pass to the command.
	Args []string `json:"args"`
}

// JobRequestStatus defines the observed state of JobRequest.

type JobRequestStatus struct {
	// Name of the Kubernetes Job created for this job request.
	JobName string `json:"jobName,omitempty"`
	// +kubebuilder:validation:Enum=Pending;Approved;Rejected;Started;Complete;Failed;Malformed
	State JobRequestState `json:"state,omitempty"`
	// Name of the JobRequestReview resource that reviewed this job request.
	ReviewName string `json:"reviewName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=jr
// +kubebuilder:printcolumn:name="Command",type=string,JSONPath=`.spec.command`
// +kubebuilder:printcolumn:name="Arguments",type=string,JSONPath=`.spec.args`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Job Name",type=string,JSONPath=`.status.jobName`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// JobRequest represents a request to run a command in the cluster.
type JobRequest struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of JobRequest
	// +required
	Spec JobRequestSpec `json:"spec"`

	// status defines the observed state of JobRequest
	// +optional
	Status JobRequestStatus `json:"status,omitzero"`
}

func (jr *JobRequest) WasReviewedBy(review *JobRequestReview) bool {
	return jr.Status.ReviewName == review.Name
}

func (jr *JobRequest) HasBeenReviewed() bool {
	if jr.Status.State == "" || jr.Status.State == JobRequestPending || jr.Status.State == JobRequestMalformed {
		return false
	} else {
		return true
	}
}

func (jr *JobRequest) HasBeenRejected() bool {
	return jr.Status.State == JobRequestRejected
}

func (jr *JobRequest) HasBeenApproved() bool {
	return jr.HasBeenReviewed() && !jr.HasBeenRejected()
}

func (jr *JobRequest) GetRequestedBy() (string, error) {
	requestedBy, ok := jr.Annotations[JobRequestRequestedByAnnotation]

	if !ok {
		return "", fmt.Errorf("JobRequest %s does not include the requested-by annotation", jr.Name)
	}

	return requestedBy, nil
}

// +kubebuilder:object:root=true

// JobRequestList contains a list of JobRequest
type JobRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []JobRequest `json:"items"`
}

<<<<<<< HEAD
func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &JobRequest{}, &JobRequestList{})
		return nil
	})
}
=======
type JobRequestState string

const (
	JobRequestPending               JobRequestState = "Pending"
	JobRequestApproved              JobRequestState = "Approved"
	JobRequestRejected              JobRequestState = "Rejected"
	JobRequestStarted               JobRequestState = "Started"
	JobRequestComplete              JobRequestState = "Complete"
	JobRequestFailed                JobRequestState = "Failed"
	JobRequestMalformed             JobRequestState = "Malformed"
	JobRequestRequestedByAnnotation string          = "platform.publishing.service.gov.uk/requested-by"
)
>>>>>>> tmp-original-10-08-26-11-02
