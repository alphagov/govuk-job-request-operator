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

type JobRequestPodSpecFrom struct {
	// +kubebuilder:validation:Enum=apps/v1;
	Group string `json:"group"`
	// +kubebuilder:validation:Enum=Deployment;
	Kind          string               `json:"kind"`
	LabelSelector metav1.LabelSelector `json:"labelSelector"`
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
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the JobRequest resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	JobName string `json:"jobName,omitempty"`
	// +kubebuilder:validation:Enum=Requested;Approved;Rejected;Started;Completed;Failed
	State string `json:"state,omitempty"`
	// Kubernetes username of the user who made the job request.
	RequestedBy string `json:"requestedBy,omitempty"`
	// Name of the JobRequestReview resource that reviewed this job request.
	ReviewName string `json:"reviewName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=jr
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.state`

// JobRequest is the Schema for the jobrequests API
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

// +kubebuilder:object:root=true

// JobRequestList contains a list of JobRequest
type JobRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []JobRequest `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JobRequest{}, &JobRequestList{})
}
