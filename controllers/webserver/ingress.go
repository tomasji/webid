package webserver

import (
	"context"

	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	webidv1alpha1 "github.com/tomasji/webid-operator/api/v1alpha1"
)

// reconcileIngress gets the ingress (NS+name is same as of the web resource)
// - if not found, create it
func (r *Reconciler) reconcileIngress(ctx context.Context, web *webidv1alpha1.WebServer) (*webidv1alpha1.WebServer, error) {
	debug := log.FromContext(ctx).V(1).Info
	nsName := types.NamespacedName{Namespace: web.Namespace, Name: web.Name}

	// Get the ingress
	debug("checking ingress", "name", web.Name)
	ingress := &netv1.Ingress{}
	if err := r.Get(ctx, nsName, ingress); err != nil {
		// generic error
		if !apierrors.IsNotFound(err) {
			return r.failWithStatus(ctx, web, err, "Failed to fetch ingress")
		}

		// ingress not found - create it
		if err = r.createIngress(ctx, web); err != nil {
			return r.failWithStatus(ctx, web, err, "Failed to create ingress")
		}
		return web, nil
	}

	// ingress found - check it and update it if needed
	debug("ingress found", "name", web.Name)
	return web, nil
}

// createIngress creates a ingress, set ownership to web
func (r *Reconciler) createIngress(ctx context.Context, web *webidv1alpha1.WebServer) error {
	const httpPort = "http"
	const indexFileName = "index.html"

	log := log.FromContext(ctx)
	labels := map[string]string{
		"app.kubernetes.io/name":    web.Name + "-nginx",
		"app.kubernetes.io/part-of": "webid-operator",
	}

	ingress := &netv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      web.Name,
			Namespace: web.Namespace,
			Labels:    labels,
		},
		Spec: netv1.IngressSpec{
			IngressClassName: &r.Cfg.IngressClass,
			Rules: []netv1.IngressRule{
				{
					Host: r.Cfg.IngressDomain,
					IngressRuleValue: netv1.IngressRuleValue{
						HTTP: &netv1.HTTPIngressRuleValue{
							Paths: []netv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: ptr(netv1.PathTypePrefix),
									Backend: netv1.IngressBackend{
										Service: &netv1.IngressServiceBackend{
											Name: web.Name,
											Port: netv1.ServiceBackendPort{Name: httpPort},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Set the ownerRef for the Ingress
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
	if err := ctrl.SetControllerReference(web, ingress, r.Scheme); err != nil {
		return err
	}

	log.Info("Creating a new Ingress", "namespace", ingress.Namespace, "name", ingress.Name)
	if err := r.Create(ctx, ingress); err != nil {
		log.Error(err, "Failed to create new Ingress", "Ingress.Namespace",
			ingress.Namespace, "Ingress.Name", ingress.Name)
		return err
	}
	log.V(1).Info("Ingress created", "namespace", ingress.Namespace, "name", ingress.Name)
	return nil
}
