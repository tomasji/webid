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

package webserver

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	webidv1alpha1 "github.com/tomasji/webid-operator/api/v1alpha1"
	"github.com/tomasji/webid-operator/controllers/config"
)

// Reconciler reconciles a WebServer object
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Cfg    *config.Config
}

const (
	typeAvailableWeb = "Available"
)

type reconcileHelperFunc = func(ctx context.Context, web *webidv1alpha1.WebServer) (*webidv1alpha1.WebServer, error)

//+kubebuilder:rbac:groups=webid.golang.betsys.com,resources=webservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=webid.golang.betsys.com,resources=webservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=webid.golang.betsys.com,resources=webservers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the WebServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	debug := log.V(1).Info

	reconcileFuncs := []reconcileHelperFunc{
		r.reconcileDeployment,
		r.reconcileConfigCM,
		r.reconcileDataCM,
		r.reconcileService,
		r.reconcileIngress,
	}

	// Get the WebServer object
	web, err := r.getObj(ctx, req.NamespacedName)
	if web == nil {
		return ctrl.Result{}, err
	}
	debug("Reconcile: got object:", "web", web)

	// Let's just set the status as Unknown when no status are available
	if web.Status.Conditions == nil || len(web.Status.Conditions) == 0 {
		if web, err = r.setStatus(ctx, web, metav1.ConditionUnknown, "Starting reconciliation"); err != nil {
			return ctrl.Result{}, err
		}
	}

	// check / create / update dependent objects
	for _, reconcileFunc := range reconcileFuncs {
		if web, err = reconcileFunc(ctx, web); err != nil {
			return ctrl.Result{}, err
		}
	}

	if web, err = r.setStatus(ctx, web, metav1.ConditionTrue, "Finished reconciliation"); err != nil {
		return ctrl.Result{}, err
	}
	debug("Reconcile: completed")
	return ctrl.Result{}, nil
}

// getObj retrieves webserver object, it returns:
// - nil, nil -> stop reconciliation (obj deleted)
// - nil, error -> stop reconciliation (requeue)
// - web, nik -> got it
func (r *Reconciler) getObj(ctx context.Context, namespacedName types.NamespacedName) (web *webidv1alpha1.WebServer, err error) {
	log := log.FromContext(ctx)

	web = &webidv1alpha1.WebServer{}
	err = r.Get(ctx, namespacedName, web)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("webserver resource not found. Ignoring since object must be deleted")
			return nil, nil
		}
		// Error reading the object - requeue the request.
		log.Error(err, "Failed to get webserver")
		return nil, err
	}
	return web, nil
}

// setStatus updates status conditions, returns the updated web object
func (r *Reconciler) setStatus(ctx context.Context, web *webidv1alpha1.WebServer, status metav1.ConditionStatus, message string,
) (updatedWeb *webidv1alpha1.WebServer, err error) {
	const statusReason = "Reconciling"
	log := log.FromContext(ctx)

	meta.SetStatusCondition(&web.Status.Conditions, metav1.Condition{Type: typeAvailableWeb, Status: status, Reason: statusReason, Message: message})
	if err = r.Status().Update(ctx, web); err != nil {
		log.Error(err, "Failed to update WebServer status")
		return nil, err
	}

	nsName := types.NamespacedName{Namespace: web.Namespace, Name: web.Name}

	// Re-fetch the Custom Resource after update the status
	if err := r.Get(ctx, nsName, web); err != nil {
		log.Error(err, "Failed to re-fetch webserver")
		return nil, err
	}
	return web, nil
}

// failWithStatus logs the error and tries to set web status.Conditions.
// returns {nil, error}
func (r *Reconciler) failWithStatus(ctx context.Context, web *webidv1alpha1.WebServer, err error, msg string) (*webidv1alpha1.WebServer, error) {
	log := log.FromContext(ctx)

	log.Error(err, msg)
	web, errStat := r.setStatus(ctx, web, metav1.ConditionFalse, "Failed to create deployment")
	if errStat != nil {
		log.Error(err, "failed to set status")
	}
	return nil, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webidv1alpha1.WebServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&netv1.Ingress{}).
		WithEventFilter(webServerEventFilter()).
		Complete(r)
}

func webServerEventFilter() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to CR status in which case metadata.Generation does not change
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration()
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Evaluates to false if the object has been confirmed deleted.
			return !e.DeleteStateUnknown
		},
	}
}

func ptr[T any](v T) *T { return &v }
