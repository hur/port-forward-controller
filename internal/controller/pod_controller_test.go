/*
Copyright 2025.

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
	"log"

	"atte.cloud/port-forward-controller/internal/forwarding"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Pod Controller", func() {
	Context("When setting up the test environment", func() {
		//It("Should install an Unifi Controller", func() {
		//	By("Installing the controller")
		//	out, err :=
		//	Expect(err).NotTo(HaveOccurred())
		//	//Expect().Should(Succeed())
		//})
		It("Should create a Pod with host port set", func() {
			By("Creating the Pod")
			const resourceName = "test-pod"

			pod := v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
					Annotations: map[string]string{
						"port-forward-controller.atte.cloud/enable": "true",
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    "nginx",
							Image:   "nginx:latest",
							Command: []string{"sleep"},
							Args:    []string{"infinity"},
							Ports: []v1.ContainerPort{
								{
									HostPort:      4773,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, &pod)).Should(Succeed())

		})
	})
	Context("When reconciling a resource", func() {
		typeNamespacedName := types.NamespacedName{
			Name:      "test-pod",
			Namespace: "default",
		}

		It("should successfully reconcile the resource", func() {
			log.Print(unifiUrl)
			log.Print("ATTE")
			client, err := forwarding.NewUnifiClient("default", "http://"+unifiUrl+":8443", "test", "test", true)
			Expect(err).NotTo(HaveOccurred())
			reconciler := PodReconciler{
				k8sClient,
				k8sClient.Scheme(),
				&forwarding.ForwardingReconciler{
					RulePrefix: "integration-test",
					Client:     client,
				},
			}

			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
