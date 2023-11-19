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

type CMType string

const (
	typeConfig CMType = "config"
	typeData   CMType = "data"
	fileConfig        = "default.conf"
	dataDir           = "/var/www"
)

func ConfigCMName(base string) string { return base + "-" + (string(typeConfig)) }
func DataCMName(base string) string   { return base + "-" + (string(typeData)) }

// reconcileConfigCM gets the configMap with nginx configuration
// - if not found, create it
func (r *Reconciler) reconcileConfigCM(ctx context.Context, web *webidv1alpha1.WebServer) (*webidv1alpha1.WebServer, error) {
	debug := log.FromContext(ctx).V(1).Info
	cmName := ConfigCMName(web.Name)
	nsName := types.NamespacedName{Namespace: web.Namespace, Name: cmName}

	// Get the config configMap
	debug("checking configMap", "name", cmName)
	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, nsName, configMap); err != nil {
		// generic error
		if !apierrors.IsNotFound(err) {
			return r.failWithStatus(ctx, web, err, "Failed to fetch configMap")
		}

		// configMap not found - create it
		if err = r.createConfigCM(ctx, web); err != nil {
			return r.failWithStatus(ctx, web, err, "Failed to create configMap")
		}
		return web, nil
	}

	// configMap found - static CM should not be updated
	debug("configMap is ok", "name", cmName)
	return web, nil
}

// reconcileDataCM gets the configMap with nginx web pages
// - if not found, create it
// - if not up to date, update it
func (r *Reconciler) reconcileDataCM(ctx context.Context, web *webidv1alpha1.WebServer) (*webidv1alpha1.WebServer, error) {
	debug := log.FromContext(ctx).V(1).Info
	cmName := DataCMName(web.Name)
	nsName := types.NamespacedName{Namespace: web.Namespace, Name: cmName}

	// Get the data configMap
	debug("checking configMap", "name", cmName)
	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, nsName, configMap); err != nil {
		// generic error
		if !apierrors.IsNotFound(err) {
			return r.failWithStatus(ctx, web, err, "Failed to fetch configMap")
		}

		// configMap not found - create it
		if err = r.createDataCM(ctx, web); err != nil {
			return r.failWithStatus(ctx, web, err, "Failed to create configMap")
		}
		return web, nil
	}

	// configMap found - check it and update it if needed
	data := r.DataProvider.GetData(types.NamespacedName{Namespace: web.Namespace, Name: web.Name})
	if r.configMapDiffers(configMap, data) {
		if err := r.updateConfigMap(ctx, configMap, data); err != nil {
			return r.failWithStatus(ctx, web, err, "Failed to update configMap")
		}
	}
	debug("configMap is ok", "name", cmName)
	return web, nil
}

// createConfigCM creates a configMap with nginx config, set ownership to web
func (r *Reconciler) createConfigCM(ctx context.Context, web *webidv1alpha1.WebServer) error {
	return r.createConfigMap(ctx, web, ConfigCMName(web.Name), map[string][]byte{fileConfig: []byte(nginxConfigData)})
}

// createDataCM creates a configMap with nginx data/web pages, set ownership to web
func (r *Reconciler) createDataCM(ctx context.Context, web *webidv1alpha1.WebServer) error {
	data := r.DataProvider.GetData(types.NamespacedName{Namespace: web.Namespace, Name: web.Name})
	return r.createConfigMap(ctx, web, DataCMName(web.Name), data)
}

// createConfigMap creates a configMap, set ownership to web
func (r *Reconciler) createConfigMap(ctx context.Context, web *webidv1alpha1.WebServer,
	name string, items map[string][]byte,
) error {
	log := log.FromContext(ctx)
	labels := map[string]string{
		"app.kubernetes.io/name":    name,
		"app.kubernetes.io/part-of": "webid-operator",
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: web.Namespace,
			Labels:    labels,
		},
		BinaryData: items,
	}

	// Set the ownerRef for the ConfigMap
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(web, configMap, r.Scheme); err != nil {
		return err
	}

	log.Info("Creating a new ConfigMap", "namespace", configMap.Namespace, "name", name)
	if err := r.Create(ctx, configMap); err != nil {
		log.Error(err, "Failed to create new ConfigMap", "ConfigMap.Namespace",
			configMap.Namespace, "ConfigMap.Name", name)
		return err
	}
	log.V(1).Info("ConfigMap created", "namespace", configMap.Namespace, "name", name)
	return nil
}

// configMapDiffers returns true if docker image or number of replicas are different than expected
func (r *Reconciler) configMapDiffers(configMap *corev1.ConfigMap, data map[string][]byte) bool {
	return r.DataProvider.DataDiffer(configMap.BinaryData, data)
}

// updateConfigMap updates image and/or replicas of the configMap
func (r *Reconciler) updateConfigMap(ctx context.Context, configMap *corev1.ConfigMap, data map[string][]byte) error {
	log := log.FromContext(ctx)

	log.Info("updating config map", "name", configMap.Name)
	configMap.BinaryData = data
	if err := r.Update(ctx, configMap); err != nil {
		return err
	}

	log.V(1).Info("config map updated", "name", configMap.Name)
	return nil
}

const nginxConfigData = `
server {
    listen       80;
    listen  [::]:80;
    server_name  localhost;

    location / {
        root   /var/www;
        autoindex on;
        autoindex_exact_size off;
        autoindex_format html;
        autoindex_localtime on;
        default_type text/html;
        index  index.html index.htm;
    }

    error_page   500 502 503 504  /50x.html;
    location = /50x.html {
        root   /usr/share/nginx/html;
    }
}
`
