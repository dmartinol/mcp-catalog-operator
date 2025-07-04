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
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mcpv1alpha1 "github.com/RHEcosystemAppEng/mcp-registry-operator/api/v1alpha1"
)

// Helper function to create McpCatalog instances
var createMcpCatalog = func(name, namespace, description, imageRegistry string) *mcpv1alpha1.McpCatalog {
	return &mcpv1alpha1.McpCatalog{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: mcpv1alpha1.McpCatalogSpec{
			Description:   description,
			ImageRegistry: imageRegistry,
		},
	}
}

var _ = Describe("McpCatalog Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		mcpcatalog := &mcpv1alpha1.McpCatalog{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind McpCatalog")
			err := k8sClient.Get(ctx, typeNamespacedName, mcpcatalog)
			if err != nil && errors.IsNotFound(err) {
				resource := createMcpCatalog(resourceName, "default", "Test catalog for server validation", "test-registry.example.com")
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &mcpv1alpha1.McpCatalog{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance McpCatalog")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource and set Ready condition to True", func() {
			By("Reconciling the created resource")
			controllerReconciler := &McpCatalogReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Wait for the status to be updated
			Eventually(func(g Gomega) {
				updatedCatalog := &mcpv1alpha1.McpCatalog{}
				err := k8sClient.Get(ctx, typeNamespacedName, updatedCatalog)
				Expect(err).NotTo(HaveOccurred())
				readyCondition := meta.FindStatusCondition(updatedCatalog.Status.Conditions, ConditionTypeReady)
				// Check that the Ready condition is set to True
				Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
				Expect(readyCondition.Reason).To(Equal(ConditionReasonValidationSucceeded))
				Expect(readyCondition.Message).To(Equal(ValidationMessageCatalogSuccess))
			}, 5*time.Second, 100*time.Millisecond).Should(Succeed())

		})
	})

	Context("When reconciling an invalid resource", func() {
		const invalidResourceName = "invalid-test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      invalidResourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating an invalid custom resource for the Kind McpCatalog")
			err := k8sClient.Get(ctx, typeNamespacedName, &mcpv1alpha1.McpCatalog{})
			if err != nil && errors.IsNotFound(err) {
				resource := createMcpCatalog(invalidResourceName, "default", "   ", "test-registry.example.com")
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &mcpv1alpha1.McpCatalog{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				By("Cleanup the invalid resource instance McpCatalog")
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("should set Ready condition to False when validation fails", func() {
			By("Reconciling the invalid resource")
			controllerReconciler := &McpCatalogReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Wait a bit for the status to be updated
			time.Sleep(100 * time.Millisecond)

			// Fetch the updated resource to check the status
			updatedCatalog := &mcpv1alpha1.McpCatalog{}
			err = k8sClient.Get(ctx, typeNamespacedName, updatedCatalog)
			Expect(err).NotTo(HaveOccurred())

			// Check that the Ready condition is set to False
			readyCondition := meta.FindStatusCondition(updatedCatalog.Status.Conditions, ConditionTypeReady)
			Expect(readyCondition).NotTo(BeNil())
			Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
			Expect(readyCondition.Reason).To(Equal(ConditionReasonValidationFailed))
			Expect(readyCondition.Message).To(ContainSubstring(ValidationMessageDescriptionRequired))
		})
	})

	Context("When validating McpCatalog resources", func() {
		It("should validate a valid McpCatalog", func() {
			controllerReconciler := &McpCatalogReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			validCatalog := createMcpCatalog("valid-catalog", "default", "A valid catalog description", "registry.example.com")

			err := controllerReconciler.validateMcpCatalog(validCatalog)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject McpCatalog with empty description", func() {
			controllerReconciler := &McpCatalogReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			invalidCatalog := createMcpCatalog("invalid-catalog", "default", "", "registry.example.com")

			err := controllerReconciler.validateMcpCatalog(invalidCatalog)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(ValidationMessageDescriptionRequired))
		})

		It("should reject McpCatalog with empty imageRegistry", func() {
			controllerReconciler := &McpCatalogReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			invalidCatalog := createMcpCatalog("invalid-catalog", "default", "A valid description", "")

			err := controllerReconciler.validateMcpCatalog(invalidCatalog)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(ValidationMessageImageRegistryRequired))
		})

		It("should reject McpCatalog with whitespace-only description", func() {
			controllerReconciler := &McpCatalogReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			invalidCatalog := &mcpv1alpha1.McpCatalog{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-catalog",
					Namespace: "default",
				},
				Spec: mcpv1alpha1.McpCatalogSpec{
					Description:   "   ",
					ImageRegistry: "registry.example.com",
				},
			}

			err := controllerReconciler.validateMcpCatalog(invalidCatalog)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(ValidationMessageDescriptionRequired))
		})

		It("should reject McpCatalog with whitespace-only imageRegistry", func() {
			controllerReconciler := &McpCatalogReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			invalidCatalog := createMcpCatalog("invalid-catalog", "default", "A valid description", "   ")

			err := controllerReconciler.validateMcpCatalog(invalidCatalog)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(ValidationMessageImageRegistryRequired))
		})

		It("should reject McpCatalog with both fields empty", func() {
			controllerReconciler := &McpCatalogReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			invalidCatalog := &mcpv1alpha1.McpCatalog{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-catalog",
					Namespace: "default",
				},
				Spec: mcpv1alpha1.McpCatalogSpec{
					Description:   "",
					ImageRegistry: "",
				},
			}

			err := controllerReconciler.validateMcpCatalog(invalidCatalog)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(ValidationMessageDescriptionRequired))
			Expect(err.Error()).To(ContainSubstring(ValidationMessageImageRegistryRequired))
		})
	})
})
