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
<<<<<<< HEAD
	// +kubebuilder:validation:Enum=Pending;Approved;Rejected;Started;Completed;Failed;Malformed
=======
	// +kubebuilder:validation:Enum=Pending;Approved;Rejected;Started;Completed;Failed
>>>>>>> 1cd9f18 (chore: 🤖 remove "requested" -> "Pending" and mv reuestedBy pls)
	State string `json:"state,omitempty"`
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

// +kubebuilder:object:root=true

// JobRequestList contains a list of JobRequest
type JobRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []JobRequest `json:"items"`
}
