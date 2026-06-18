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

		FIt("should successfully reconcile the resource", func() {
			By("Reconciling the created primary resource")

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

			Expect(jobList.Items[0].ObjectMeta.GetOwnerReferences()).NotTo(BeNil())
			Expect(jobList.Items[0].ObjectMeta.GetOwnerReferences()[0].Name).To(Equal(resourceName))
			Expect(jobList.Items[0].ObjectMeta.GetOwnerReferences()[0].Kind).To(Equal("JobRequest"))

			By("Cleanup the JobRequest, Deployment and Job")
			Expect(k8sClient.Delete(ctx, targetResource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &jobList.Items[0])).To(Succeed())
		})

		It("should successfully reconcile if jobRequest doesn't exist", func() {
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

		It("should successfully reconcile if we cannot retrieve the target resource in the jobRequest from the cluster", func() {
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

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the JobRequest")
			Expect(k8sClient.Delete(ctx, jobRequest)).To(Succeed())
		})

		// TODO: Rewrite test name
		It("should successfully reconcile even if the container in the resource reference from the jobRequest doesn't exist", func() {
			var replicasNum int32 = 1
			containerName := "example-container"
			targetContainerName := "foo-container"
			resourceName := "test-resource"
			resourceNamespace := "default"

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
						ContainerName: containerName,
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
									Name:  targetContainerName,
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

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the JobRequest and Deployment")
			Expect(k8sClient.Delete(ctx, jobRequest)).To(Succeed())
			Expect(k8sClient.Delete(ctx, targetResource)).To(Succeed())
		})

		It("should return an error when the cacheClient cannot create the job", func() {
			Skip("todo")
			// Try see if creating it into a namespace that doesn't exist
		})
	})
})
