/*
Copyright 2023.

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
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	vpnv1alpha1 "github.com/dusty-cjh/v2ray-operator/api/v1alpha1"
)

var _ = Describe("V2rayUser controller", func() {
	Context("V2rayUser controller test", func() {

		const V2rayUserName = "test-v2rayuser"

		ctx := context.Background()

		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:      V2rayUserName,
				Namespace: V2rayUserName,
			},
		}

		typeNamespaceName := types.NamespacedName{Name: V2rayUserName, Namespace: V2rayUserName}

		BeforeEach(func() {
			By("Creating the Namespace to perform the tests")
			err := k8sClient.Create(ctx, namespace)
			Expect(err).To(Not(HaveOccurred()))

			By("Setting the Image ENV VAR which stores the Operand image")
			err = os.Setenv("V2RAYUSER_IMAGE", "example.com/image:test")
			Expect(err).To(Not(HaveOccurred()))
		})

		AfterEach(func() {
			// TODO(user): Attention if you improve this code by adding other context test you MUST
			// be aware of the current delete namespace limitations. More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
			By("Deleting the Namespace to perform the tests")
			_ = k8sClient.Delete(ctx, namespace)

			By("Removing the Image ENV VAR which stores the Operand image")
			_ = os.Unsetenv("V2RAYUSER_IMAGE")
		})

		It("should successfully reconcile a custom resource for V2rayUser", func() {
			By("Creating the custom resource for the Kind V2rayUser")
			v2rayuser := &vpnv1alpha1.V2rayUser{}
			err := k8sClient.Get(ctx, typeNamespaceName, v2rayuser)
			if err != nil && errors.IsNotFound(err) {
				// Let's mock our custom resource at the same way that we would
				// apply on the cluster the manifest under config/samples
				v2rayuser := &vpnv1alpha1.V2rayUser{
					ObjectMeta: metav1.ObjectMeta{
						Name:      V2rayUserName,
						Namespace: namespace.Name,
					},
					Spec: vpnv1alpha1.V2rayUserSpec{
						User: vpnv1alpha1.V2rayUserInfo{
							Id:    "2f88007a-eb8c-b665-3f83-4448545b027f",
							Email: "test1it@qq.com",
						},
						InboundTag: "ws+vless",
						NodeList: map[string]*vpnv1alpha1.V2rayNodeList{
							"us": {
								Size: 1,
							},
						},
					},
				}

				err = k8sClient.Create(ctx, v2rayuser)
				Expect(err).To(Not(HaveOccurred()))
			}

			By("Checking if the custom resource was successfully created")
			Eventually(func() error {
				found := &vpnv1alpha1.V2rayUser{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Reconciling the custom resource created")
			v2rayuserReconciler := &V2rayUserReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err = v2rayuserReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespaceName,
			})
			Expect(err).To(Not(HaveOccurred()))

			By("Checking if Deployment was successfully created in the reconciliation")
			Eventually(func() error {
				found := &appsv1.Deployment{}
				return k8sClient.Get(ctx, typeNamespaceName, found)
			}, time.Minute, time.Second).Should(Succeed())

			By("Checking the latest Status Condition added to the V2rayUser instance")
			Eventually(func() error {
				if v2rayuser.Status.Conditions != nil && len(v2rayuser.Status.Conditions) != 0 {
					latestStatusCondition := v2rayuser.Status.Conditions[len(v2rayuser.Status.Conditions)-1]
					expectedLatestStatusCondition := metav1.Condition{Type: typeAvailableV2rayUser,
						Status: metav1.ConditionTrue, Reason: "Reconciling",
						Message: fmt.Sprintf("Deployment for custom resource (%s) replicas created successfully", v2rayuser.Name)}
					if latestStatusCondition != expectedLatestStatusCondition {
						return fmt.Errorf("The latest status condition added to the v2rayuser instance is not as expected")
					}
				}
				return nil
			}, time.Minute, time.Second).Should(Succeed())
		})
	})
})
