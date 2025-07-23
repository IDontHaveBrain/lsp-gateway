package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// DeploymentController manages Kubernetes deployments
type DeploymentController struct {
	clientset kubernetes.Interface
	namespace string
}

// NewDeploymentController creates a new deployment controller
func NewDeploymentController(clientset kubernetes.Interface, namespace string) *DeploymentController {
	return &DeploymentController{
		clientset: clientset,
		namespace: namespace,
	}
}

// CreateDeployment creates a new deployment with the given specifications
func (dc *DeploymentController) CreateDeployment(ctx context.Context, name string, replicas int32, image string) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: dc.namespace,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: image,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
	}

	result, err := dc.clientset.AppsV1().Deployments(dc.namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create deployment %s: %w", name, err)
	}

	return result, nil
}

// GetDeployment retrieves a deployment by name
func (dc *DeploymentController) GetDeployment(ctx context.Context, name string) (*appsv1.Deployment, error) {
	deployment, err := dc.clientset.AppsV1().Deployments(dc.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s: %w", name, err)
	}
	return deployment, nil
}

// UpdateDeploymentReplicas updates the replica count for a deployment
func (dc *DeploymentController) UpdateDeploymentReplicas(ctx context.Context, name string, replicas int32) error {
	deployment, err := dc.GetDeployment(ctx, name)
	if err != nil {
		return err
	}

	deployment.Spec.Replicas = &replicas

	_, err = dc.clientset.AppsV1().Deployments(dc.namespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update deployment %s replicas: %w", name, err)
	}

	return nil
}

// ListDeployments returns all deployments in the namespace
func (dc *DeploymentController) ListDeployments(ctx context.Context) (*appsv1.DeploymentList, error) {
	deployments, err := dc.clientset.AppsV1().Deployments(dc.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}
	return deployments, nil
}

// DeleteDeployment deletes a deployment by name
func (dc *DeploymentController) DeleteDeployment(ctx context.Context, name string) error {
	err := dc.clientset.AppsV1().Deployments(dc.namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete deployment %s: %w", name, err)
	}
	return nil
}

// WaitForDeploymentReady waits for a deployment to become ready
func (dc *DeploymentController) WaitForDeploymentReady(ctx context.Context, name string, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for deployment %s to become ready", name)
		default:
			deployment, err := dc.GetDeployment(ctx, name)
			if err != nil {
				return err
			}

			if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
				return nil
			}

			time.Sleep(2 * time.Second)
		}
	}
}

// Interface demonstrates interface implementation
type DeploymentManager interface {
	CreateDeployment(ctx context.Context, name string, replicas int32, image string) (*appsv1.Deployment, error)
	GetDeployment(ctx context.Context, name string) (*appsv1.Deployment, error)
	UpdateDeploymentReplicas(ctx context.Context, name string, replicas int32) error
	DeleteDeployment(ctx context.Context, name string) error
}

// Ensure DeploymentController implements DeploymentManager
var _ DeploymentManager = (*DeploymentController)(nil)

// HealthChecker provides health checking functionality
type HealthChecker struct {
	controller *DeploymentController
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(controller *DeploymentController) *HealthChecker {
	return &HealthChecker{
		controller: controller,
	}
}

// CheckDeploymentHealth checks the health of a deployment
func (hc *HealthChecker) CheckDeploymentHealth(ctx context.Context, name string) (bool, error) {
	deployment, err := hc.controller.GetDeployment(ctx, name)
	if err != nil {
		return false, err
	}

	// Check if deployment is ready
	if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
		return true, nil
	}

	return false, nil
}

// Constants for testing
const (
	DefaultNamespace = "default"
	DefaultTimeout   = 5 * time.Minute
	MaxRetries       = 3
)

// DeploymentStatus represents deployment status
type DeploymentStatus string

const (
	StatusPending   DeploymentStatus = "Pending"
	StatusRunning   DeploymentStatus = "Running"
	StatusCompleted DeploymentStatus = "Completed"
	StatusFailed    DeploymentStatus = "Failed"
)

// StatusFromString converts string to DeploymentStatus
func StatusFromString(status string) DeploymentStatus {
	switch status {
	case string(StatusPending):
		return StatusPending
	case string(StatusRunning):
		return StatusRunning
	case string(StatusCompleted):
		return StatusCompleted
	case string(StatusFailed):
		return StatusFailed
	default:
		return StatusPending
	}
}
