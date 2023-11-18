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

// reconcileService gets the service (NS+name is same as of the web resource)
// - if not found, create it
func (r *Reconciler) reconcileService(ctx context.Context, web *webidv1alpha1.WebServer) (*webidv1alpha1.WebServer, error) {
	debug := log.FromContext(ctx).V(1).Info
	nsName := types.NamespacedName{Namespace: web.Namespace, Name: web.Name}

	// Get the service
	debug("checking service", "name", web.Name)
	service := &corev1.Service{}
	if err := r.Get(ctx, nsName, service); err != nil {
		// generic error
		if !apierrors.IsNotFound(err) {
			return r.failWithStatus(ctx, web, err, "Failed to fetch service")
		}

		// service not found - create it
		if err = r.createService(ctx, web); err != nil {
			return r.failWithStatus(ctx, web, err, "Failed to create service")
		}
		return web, nil
	}

	// service found - check it and update it if needed
	debug("service found", "name", web.Name)
	return web, nil
}

// createService creates a service, set ownership to web
func (r *Reconciler) createService(ctx context.Context, web *webidv1alpha1.WebServer) error {
	const httpPort = "http"
	const indexFileName = "index.html"

	log := log.FromContext(ctx)
	labels := map[string]string{
		"app.kubernetes.io/name":    web.Name + "-nginx",
		"app.kubernetes.io/part-of": "webid-operator",
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      web.Name,
			Namespace: web.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: httpPort,
					Port: 80,
				},
			},
			Selector: r.selectorLabels(web.Name),
		},
	}

	// Set the ownerRef for the Service
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(web, service, r.Scheme); err != nil {
		return err
	}

	log.Info("Creating a new Service", "namespace", service.Namespace, "name", service.Name)
	if err := r.Create(ctx, service); err != nil {
		log.Error(err, "Failed to create new Service", "Service.Namespace",
			service.Namespace, "Service.Name", service.Name)
		return err
	}
	log.V(1).Info("Service created", "namespace", service.Namespace, "name", service.Name)
	return nil
}
