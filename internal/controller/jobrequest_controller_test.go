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

package controller

import (
	"context"

	"k8s.io/utils/ptr"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appsv1 "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1 "github.com/alphagov/govuk-job-request-operator/api/v1"
)

var _ = Describe("JobRequest Controller", func() {
	Context("When reconciling a resource", func() {
		resourceName := "sample"
		resourceNamespace := "default"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}

		It("should successfully reconcile. The JobRequest resource should be 'Approved' and the job created", func() {
			var replicasNum int32 = 1

			jobRequestStatus := platformv1.JobRequestStatus{
				JobName:    resourceName,
				State:      "Approved",
				ReviewName: "test",
			}

			jobRequest := &platformv1.JobRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: platformv1.JobRequestSpec{
					ContainerFrom: platformv1.JobRequestContainerFrom{
						PodSpecFrom: platformv1.JobRequestPodSpecFrom{
							Group: "apps/v1",
							Kind:  "Deployment",
							Name:  resourceName,
						},
						ContainerName: "foo-container",
					},
					Command: "echo",
					Args:    []string{"Hello, World!"},
				},
			}

			targetResource := &appsv1.Deployment{
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
									Name:  "foo-container",
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

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

			actualApprovedJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualApprovedJobRequest)

			Expect(actualApprovedJobRequest.Status.State).To(Equal("Approved"))

			controllerReconciler := &JobRequestReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			jobList := &batch.JobList{}
			opts := []client.ListOption{
				client.MatchingFields{"metadata.name": resourceName},
			}
			k8sApiReader.List(ctx, jobList, opts...)

			Expect(len(jobList.Items)).To(BeNumerically("==", 1))
			Expect(jobList.Items[0].GetName()).To(Equal(resourceName))
			Expect(jobList.Items[0].GetNamespace()).To(Equal(resourceNamespace))
			Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].Name).To(Equal("foo-container"))
			Expect(len(jobList.Items[0].Spec.Template.Spec.Containers)).To(BeNumerically("==", 1))
			Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].Image).To(Equal("foo/bar"))
			Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].Env[0].Name).To(Equal("foo"))
			Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].Env[0].Value).To(Equal("bar"))
			Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].SecurityContext.AllowPrivilegeEscalation).To(Equal(ptr.To(false)))
			Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].SecurityContext.Capabilities.Drop[0]).To(BeEquivalentTo("all"))
			Expect(jobList.Items[0].Spec.Template.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem).To(Equal(ptr.To(true)))
			Expect(jobList.Items[0].Spec.Template.Spec.SecurityContext.RunAsNonRoot).To(Equal(ptr.To(true)))
			Expect(jobList.Items[0].Spec.Template.Spec.SecurityContext.RunAsUser).To(Equal(ptr.To(int64(1001))))
			Expect(jobList.Items[0].Spec.Template.Spec.SecurityContext.RunAsGroup).To(Equal(ptr.To(int64(1001))))
			Expect(jobList.Items[0].Spec.Template.Spec.SecurityContext.FSGroup).To(Equal(ptr.To(int64(1001))))
			Expect(jobList.Items[0].Spec.Template.Spec.SecurityContext.SeccompProfile.Type).To(BeEquivalentTo("RuntimeDefault"))
			Expect(jobList.Items[0].Spec.Template.Spec.RestartPolicy).To(Equal(v1.RestartPolicyNever))
			Expect(jobList.Items[0].Annotations["foo"]).To(Equal("bar"))
			Expect(jobList.Items[0].Labels["fizz"]).To(Equal("buzz"))

			actualStartedJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualStartedJobRequest)

			Expect(actualStartedJobRequest.Status.State).To(Equal("Started"))

			Expect(jobList.Items[0].ObjectMeta.GetOwnerReferences()).NotTo(BeNil())
			Expect(jobList.Items[0].ObjectMeta.GetOwnerReferences()[0].Name).To(Equal(resourceName))
			Expect(jobList.Items[0].ObjectMeta.GetOwnerReferences()[0].Kind).To(Equal("JobRequest"))

			By("Cleanup the JobRequest, Deployment and Job")
			Expect(k8sClient.Delete(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, jobRequest)).To(Succeed())

			var foreground metav1.DeletionPropagation = "Background"
			var noGrace int64 = 0
			deleteErr := k8sClient.Delete(ctx, &jobList.Items[0], &client.DeleteOptions{PropagationPolicy: &foreground, GracePeriodSeconds: &noGrace})
			Expect(deleteErr).To(BeNil())
		})

		It("should successfully reconcile if JobRequest doesn't exist and the job should not be created", func() {
			controllerReconciler := &JobRequestReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			Expect(err).NotTo(HaveOccurred())
		})

		It("should successfully reconcile if we cannot retrieve the target resource in the JobRequest from the cluster and the job should not be created", func() {
			jobRequest := &platformv1.JobRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: platformv1.JobRequestSpec{
					ContainerFrom: platformv1.JobRequestContainerFrom{
						PodSpecFrom: platformv1.JobRequestPodSpecFrom{
							Group: "apps/v1",
							Kind:  "Deployment",
							Name:  "example-app",
						},
						ContainerName: "example-container",
					},
					Command: "echo",
					Args:    []string{"Hello, World!"},
				},
			}

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			controllerReconciler := &JobRequestReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			actualJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualJobRequest)

			Expect(actualJobRequest.Status.State).To(Equal(""))

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			actualFailedJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualFailedJobRequest)

			Expect(actualFailedJobRequest.Status.State).To(Equal("Failed"))

			By("Cleanup the JobRequest")
			Expect(k8sClient.Delete(ctx, jobRequest)).To(Succeed())
		})

		It("should successfully reconcile even if the the target container in the target resource used to create the job from doesn't exist and don't create the job", func() {
			var replicasNum int32 = 1

			jobRequest := &platformv1.JobRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: platformv1.JobRequestSpec{
					ContainerFrom: platformv1.JobRequestContainerFrom{
						PodSpecFrom: platformv1.JobRequestPodSpecFrom{
							Group: "apps/v1",
							Kind:  "Deployment",
							Name:  resourceName,
						},
						ContainerName: "non-existent-container",
					},
					Command: "echo",
					Args:    []string{"Hello, World!"},
				},
			}

			targetResource := &appsv1.Deployment{
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
							Containers: []v1.Container{
								{
									Name:  "not-this-one",
									Image: "foo/bar",
									Env: []v1.EnvVar{
										{
											Name:  "foo",
											Value: "bar",
										},
									},
								},
							},
						},
					},
				},
			}

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())

			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			controllerReconciler := &JobRequestReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			actualJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualJobRequest)

			Expect(actualJobRequest.Status.State).To(Equal(""))

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			Expect(err).NotTo(HaveOccurred())

			actualFailedJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualFailedJobRequest)

			Expect(actualFailedJobRequest.Status.State).To(Equal("Failed"))

			By("Cleanup the JobRequest and Deployment")
			Expect(k8sClient.Delete(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, jobRequest)).To(Succeed())
		})

		It("should successfully reconcile. The JobRequest resource should be 'Pending' and the job should not be created yet", func() {
			var replicasNum int32 = 1

			jobRequest := &platformv1.JobRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: platformv1.JobRequestSpec{
					ContainerFrom: platformv1.JobRequestContainerFrom{
						PodSpecFrom: platformv1.JobRequestPodSpecFrom{
							Group: "apps/v1",
							Kind:  "Deployment",
							Name:  resourceName,
						},
						ContainerName: "foo-container",
					},
					Command: "echo",
					Args:    []string{"Hello, World!"},
				},
			}

			targetResource := &appsv1.Deployment{
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
									Name:  "foo-container",
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

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())

			actualJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualJobRequest)

			Expect(actualJobRequest.Status.State).To(Equal(""))

			controllerReconciler := &JobRequestReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			jobList := &batch.JobList{}
			opts := []client.ListOption{
				client.MatchingFields{"metadata.name": resourceName},
			}
			k8sApiReader.List(ctx, jobList, opts...)

			Expect(len(jobList.Items)).To(BeNumerically("==", 0))

			actualPendingJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualPendingJobRequest)

			Expect(actualPendingJobRequest.Status.State).To(Equal("Pending"))

			By("Cleanup the JobRequest, Deployment and Job")
			Expect(k8sClient.Delete(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, jobRequest)).To(Succeed())
		})

		It("should successfully reconcile. The JobRequest resource should be 'Rejected' and the job should not be created", func() {
			var replicasNum int32 = 1

			jobRequestStatus := platformv1.JobRequestStatus{
				JobName:    resourceName,
				State:      "Rejected",
				ReviewName: "test",
			}

			jobRequest := &platformv1.JobRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: platformv1.JobRequestSpec{
					ContainerFrom: platformv1.JobRequestContainerFrom{
						PodSpecFrom: platformv1.JobRequestPodSpecFrom{
							Group: "apps/v1",
							Kind:  "Deployment",
							Name:  resourceName,
						},
						ContainerName: "foo-container",
					},
					Command: "echo",
					Args:    []string{"Hello, World!"},
				},
			}

			targetResource := &appsv1.Deployment{
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
									Name:  "foo-container",
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

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

			actualJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualJobRequest)

			Expect(actualJobRequest.Status.State).To(Equal("Rejected"))

			controllerReconciler := &JobRequestReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			jobList := &batch.JobList{}
			opts := []client.ListOption{
				client.MatchingFields{"metadata.name": resourceName},
			}
			k8sApiReader.List(ctx, jobList, opts...)

			Expect(len(jobList.Items)).To(BeNumerically("==", 0))

			actualRejectedJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualRejectedJobRequest)

			Expect(actualRejectedJobRequest.Status.State).To(Equal("Rejected"))

			By("Cleanup the JobRequest, Deployment and Job")
			Expect(k8sClient.Delete(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, jobRequest)).To(Succeed())
		})

		It("should successfully reconcile. The JobRequest resource should be 'Started' and the job should not be created again", func() {
			var replicasNum int32 = 1

			jobRequestStatus := platformv1.JobRequestStatus{
				JobName:    resourceName,
				State:      "Approved",
				ReviewName: "test",
			}

			jobRequest := &platformv1.JobRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: platformv1.JobRequestSpec{
					ContainerFrom: platformv1.JobRequestContainerFrom{
						PodSpecFrom: platformv1.JobRequestPodSpecFrom{
							Group: "apps/v1",
							Kind:  "Deployment",
							Name:  resourceName,
						},
						ContainerName: "foo-container",
					},
					Command: "echo",
					Args:    []string{"Hello, World!"},
				},
			}

			targetResource := &appsv1.Deployment{
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
									Name:  "foo-container",
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

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

			actualApprovedJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualApprovedJobRequest)

			Expect(actualApprovedJobRequest.Status.State).To(Equal("Approved"))

			controllerReconciler := &JobRequestReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			jobList := &batch.JobList{}
			opts := []client.ListOption{
				client.MatchingFields{"metadata.name": resourceName},
			}
			k8sApiReader.List(ctx, jobList, opts...)

			Expect(len(jobList.Items)).To(BeNumerically("==", 1))

			actualStartedJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualStartedJobRequest)

			Expect(actualStartedJobRequest.Status.State).To(Equal("Started"))

			_, reconcileErr := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(reconcileErr).NotTo(HaveOccurred())

			secondJobList := &batch.JobList{}
			k8sApiReader.List(ctx, secondJobList, opts...)

			Expect(len(jobList.Items)).To(BeNumerically("==", 1))

			By("Cleanup the JobRequest, Deployment and Job")
			Expect(k8sClient.Delete(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, jobRequest)).To(Succeed())

			var foreground metav1.DeletionPropagation = "Background"
			var noGrace int64 = 0
			deleteErr := k8sClient.Delete(ctx, &jobList.Items[0], &client.DeleteOptions{PropagationPolicy: &foreground, GracePeriodSeconds: &noGrace})
			Expect(deleteErr).To(BeNil())
		})

		It("should successfully reconcile. The JobRequest resource should be 'Failed' and the job should not be created", func() {
			var replicasNum int32 = 1

			jobRequestStatus := platformv1.JobRequestStatus{
				JobName:    resourceName,
				State:      "Failed",
				ReviewName: "test",
			}

			jobRequest := &platformv1.JobRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
				},
				Spec: platformv1.JobRequestSpec{
					ContainerFrom: platformv1.JobRequestContainerFrom{
						PodSpecFrom: platformv1.JobRequestPodSpecFrom{
							Group: "apps/v1",
							Kind:  "Deployment",
							Name:  resourceName,
						},
						ContainerName: "foo-container",
					},
					Command: "echo",
					Args:    []string{"Hello, World!"},
				},
			}

			targetResource := &appsv1.Deployment{
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
									Name:  "foo-container",
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

			Expect(k8sClient.Create(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Create(ctx, jobRequest)).To(Succeed())
			jobRequest.Status = jobRequestStatus
			Expect(k8sClient.Status().Update(ctx, jobRequest)).To(Succeed())

			actualJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualJobRequest)

			Expect(actualJobRequest.Status.State).To(Equal("Failed"))

			controllerReconciler := &JobRequestReconciler{
				CacheClient:     k8sClient,
				ApiServerClient: k8sApiReader,
				Scheme:          k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			jobList := &batch.JobList{}
			opts := []client.ListOption{
				client.MatchingFields{"metadata.name": resourceName},
			}
			k8sApiReader.List(ctx, jobList, opts...)

			Expect(len(jobList.Items)).To(BeNumerically("==", 0))

			actualFailedJobRequest := &platformv1.JobRequest{}
			k8sClient.Get(ctx, typeNamespacedName, actualFailedJobRequest)

			Expect(actualFailedJobRequest.Status.State).To(Equal("Failed"))

			By("Cleanup the JobRequest, Deployment and Job")
			Expect(k8sClient.Delete(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, jobRequest)).To(Succeed())
		})
	})
})
