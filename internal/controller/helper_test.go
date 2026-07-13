package controller

import (
	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func jobRequestBuilder(jobRequestName, resourceName, resourceNamespace, containerName string) *platformv1.JobRequest {
	return &platformv1.JobRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobRequestName,
			Namespace: resourceNamespace,
			Annotations: map[string]string{
				"platform.publishing.service.gov.uk/requested-by": "arn:aws:sts::123456789:assumed-role/user.name-platformengineer/environment-platformengineer",
			},
		},
		Spec: platformv1.JobRequestSpec{
			ContainerFrom: platformv1.JobRequestContainerFrom{
				PodSpecFrom: platformv1.JobRequestPodSpecFrom{
					Group: "apps/v1",
					Kind:  "Deployment",
					Name:  resourceName,
				},
				ContainerName: containerName,
			},
			Command: "echo",
			Args:    []string{"Hello, World!"},
		},
	}
}

func jobRequestReviewBuilder(jobRequestName, resourceNamespace, jobRequestReviewName, decision string) *platformv1.JobRequestReview {
	return &platformv1.JobRequestReview{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobRequestReviewName,
			Namespace: resourceNamespace,
			Annotations: map[string]string{
				"platform.publishing.service.gov.uk/reviewed-by": "arn:aws:sts::123456789:assumed-role/user.name-platformengineer/environment-platformengineer",
			},
		},
		Spec: platformv1.JobRequestReviewSpec{
			JobRequestName: jobRequestName,
			Decision:       decision,
			Description:    "A description",
		},
	}
}

func deploymentBuilder(resourceName, resourceNamespace string) *appsv1.Deployment {
	var replicasNum int32 = 1
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceName,
			Namespace: resourceNamespace,
			Annotations: map[string]string{
				"foo": "bar",
			},
			Labels: map[string]string{
				"fizz": "buzz",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicasNum,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "foo",
				},
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "foo",
					},
				},
				Spec: v1.PodSpec{
					RestartPolicy: "Always",
					SecurityContext: &v1.PodSecurityContext{
						RunAsUser:    ptr.To(int64(1001)),
						RunAsGroup:   ptr.To(int64(1001)),
						FSGroup:      ptr.To(int64(1001)),
						RunAsNonRoot: ptr.To(true),
						SeccompProfile: &v1.SeccompProfile{
							Type: "RuntimeDefault",
						},
					},
					Containers: []v1.Container{
						{
							Name:  "foo",
							Image: "foo/bar",
							Env: []v1.EnvVar{
								{
									Name:  "foo",
									Value: "bar",
								},
							},
							SecurityContext: &v1.SecurityContext{
								AllowPrivilegeEscalation: ptr.To(false),
								Capabilities: &v1.Capabilities{
									Drop: []v1.Capability{
										"all",
									},
								},
								ReadOnlyRootFilesystem: ptr.To(true),
							},
						},
					},
				},
			},
		},
	}
}
