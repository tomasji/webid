package webserver

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	webidv1alpha1 "github.com/tomasji/webid-operator/api/v1alpha1"
)

// reconcileDeployment gets the deployment (NS+name is same as of the web resource)
// - if not found, create it
// - if found, compare it with the required status, update if necessary
func (r *WebServerReconciler) reconcileDeployment(ctx context.Context, web *webidv1alpha1.WebServer) (*webidv1alpha1.WebServer, error) {
	debug := log.FromContext(ctx).V(1).Info
	nsName := types.NamespacedName{Namespace: web.Namespace, Name: web.Name}

	// Get the deployment
	debug("checking deployment", "name", web.Name)
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, nsName, deployment); err != nil {
		// generic error
		if !apierrors.IsNotFound(err) {
			return r.failWithStatus(ctx, web, err, "Failed to fetch deployment")
		}

		// deployment not found - create it
		if err = r.createDeployment(ctx, web); err != nil {
			return r.failWithStatus(ctx, web, err, "Failed to create deployment")
		}
		return web, nil
	}

	// deployment found - check it and update it if needed
	debug("deployment found", "name", web.Name)
	if r.deploymentDiffers(web, deployment) {
		if err := r.updateDeployment(ctx, web, deployment); err != nil {
			return r.failWithStatus(ctx, web, err, "Failed to update deployment")
		}
	} else {
		debug("deployment is ok", "name", web.Name)
	}
	return web, nil
}

func (r *WebServerReconciler) selectorLabels(appName string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/name":    appName + "-nginx",
		"app.kubernetes.io/part-of": "webid-operator",
	}
}

// createDeployment creates a deployment, set ownership to web
func (r *WebServerReconciler) createDeployment(ctx context.Context, web *webidv1alpha1.WebServer) error {
	const volName = "config"
	const configMountPath = "/etc/web"

	log := log.FromContext(ctx)
	labels := r.selectorLabels(web.Name)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      web.Name,
			Namespace: web.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr(web.Spec.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:           web.Spec.Image,
						Name:            "main",
						ImagePullPolicy: corev1.PullIfNotPresent,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 80,
							Name:          "http",
						}},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      volName,
								ReadOnly:  true,
								MountPath: configMountPath,
							},
						},
					}},
					Volumes: []corev1.Volume{
						{
							Name: volName,
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: web.Name,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Set the ownerRef for the Deployment
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(web, deployment, r.Scheme); err != nil {
		return err
	}

	log.Info("Creating a new Deployment", "namespace", deployment.Namespace, "name", deployment.Name)
	if err := r.Create(ctx, deployment); err != nil {
		log.Error(err, "Failed to create new Deployment", "Deployment.Namespace",
			deployment.Namespace, "Deployment.Name", deployment.Name)
		return err
	}
	return nil
}

// deploymentDiffers returns true if docker image or number of replicas are different than expected
func (r *WebServerReconciler) deploymentDiffers(web *webidv1alpha1.WebServer, deployment *appsv1.Deployment) bool {
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		return true
	}
	return web.Spec.Image != deployment.Spec.Template.Spec.Containers[0].Image ||
		web.Spec.Replicas != *deployment.Spec.Replicas
}

// updateDeployment updates image and/or replicas of the deployment
func (r *WebServerReconciler) updateDeployment(ctx context.Context, web *webidv1alpha1.WebServer, deployment *appsv1.Deployment) error {
	log := log.FromContext(ctx)

	log.Info("updating deployment", "name", web.Name)
	if len(deployment.Spec.Template.Spec.Containers) != 1 { // should never happen
		if delErr := r.Delete(ctx, deployment); delErr != nil {
			log.Error(delErr, "deleting deployment")
		}
		return fmt.Errorf("deployment '%s' has %d containers (expected 1)", deployment.Name, len(deployment.Spec.Template.Spec.Containers))
	}

	deployment.Spec.Template.Spec.Containers[0].Image = web.Spec.Image
	deployment.Spec.Replicas = ptr(web.Spec.Replicas)
	if err := r.Update(ctx, deployment); err != nil {
		return err
	}

	log.V(1).Info("deployment updated", "name", deployment.Name)
	return nil
}
