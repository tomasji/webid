
### init project
```
operator-sdk init --domain golang.betsys.com --repo github.com/tomasji/webid-operator
```

### Create a new API and Controller
```
operator-sdk create api --group webid --version v1alpha1 --kind WebServer --resource --controller
operator-sdk create api --group webid --version v1alpha1 --kind Page      --resource --controller
```

- edit the generated API `api/v1alpha1/*.go`
- After modifying the `*_types.go` file, run: `make generate` (api/v1alpha1/zz_generated.deepcopy.go)
- for comment markers see:
  - https://book.kubebuilder.io/reference/markers/crd-validation.html
  - https://sdk.operatorframework.io/docs/building-operators/golang/references/markers/

example:
```
type WebServerSpec struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1

	// Replicas defines the number of WebID instances
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Number of pods"
	Replicas uint32 `json:"replicas,omitempty"`
}
```

### Generating CRD manifests
```
make manifests
# -> config/crd/bases/webid.golang.betsys.com_webservers.yaml
#    config/rbac/role.yaml
```

### Implement the Controller
`controllers/webserver_controller.go`
- Resources watched by the Controller: add `Owns(&appsv1.Deployment{})` if needed
  - the ownership must be recorded in the Deployment objects (`ctrl.SetControllerReference()`)
- Filter events if needed: add `WithEventFilter(webServerEventFilter())`.
```
appsv1 "k8s.io/api/apps/v1"

func (r *WebServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&webidv1alpha1.WebServer{}).
		Owns(&appsv1.Deployment{}).
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
```
