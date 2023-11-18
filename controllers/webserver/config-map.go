package webserver

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	webidv1alpha1 "github.com/tomasji/webid-operator/api/v1alpha1"
)

// reconcileConfigMap gets the configMap (NS+name is same as of the web resource)
// - if not found, create it
// - if found, compare it with the required status, update if necessary
func (r *WebServerReconciler) reconcileConfigMap(ctx context.Context, web *webidv1alpha1.WebServer) (*webidv1alpha1.WebServer, error) {
	debug := log.FromContext(ctx).V(1).Info
	nsName := types.NamespacedName{Namespace: web.Namespace, Name: web.Name}

	// Get the configMap
	debug("checking configMap", "name", web.Name)
	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, nsName, configMap); err != nil {
		// generic error
		if !apierrors.IsNotFound(err) {
			return r.failWithStatus(ctx, web, err, "Failed to fetch configMap")
		}

		// configMap not found - create it
		if err = r.createConfigMap(ctx, web); err != nil {
			return r.failWithStatus(ctx, web, err, "Failed to create configMap")
		}
		return web, nil
	}

	// configMap found - check it and update it if needed
	debug("configMap found", "name", web.Name)
	if r.configMapDiffers(web, configMap) {
		if err := r.updateConfigMap(ctx, web, configMap); err != nil {
			return r.failWithStatus(ctx, web, err, "Failed to update configMap")
		}
	} else {
		debug("configMap is ok", "name", web.Name)
	}
	return web, nil
}

// createConfigMap creates a configMap, set ownership to web
func (r *WebServerReconciler) createConfigMap(ctx context.Context, web *webidv1alpha1.WebServer) error {
	const indexFileName = "index.html"

	log := log.FromContext(ctx)
	labels := map[string]string{
		"app.kubernetes.io/name":    web.Name + "-nginx",
		"app.kubernetes.io/part-of": "webid-operator",
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      web.Name,
			Namespace: web.Namespace,
			Labels:    labels,
		},
		BinaryData: map[string][]byte{
			indexFileName: []byte("Hello"),
		},
	}

	// Set the ownerRef for the ConfigMap
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(web, configMap, r.Scheme); err != nil {
		return err
	}

	log.Info("Creating a new ConfigMap", "namespace", configMap.Namespace, "name", configMap.Name)
	if err := r.Create(ctx, configMap); err != nil {
		log.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace",
			configMap.Namespace, "ConfigMap.Name", configMap.Name)
		return err
	}
	log.V(1).Info("ConfigMap created", "namespace", configMap.Namespace, "name", configMap.Name)
	return nil
}

// configMapDiffers returns true if docker image or number of replicas are different than expected
func (r *WebServerReconciler) configMapDiffers(web *webidv1alpha1.WebServer, configMap *corev1.ConfigMap) bool {
	return false // TODO: implement
}

// updateConfigMap updates image and/or replicas of the configMap
func (r *WebServerReconciler) updateConfigMap(ctx context.Context, web *webidv1alpha1.WebServer, configMap *corev1.ConfigMap) error {
	return nil // TODO: implement
}
