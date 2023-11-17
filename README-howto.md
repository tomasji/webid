
### init project
```
operator-sdk init --domain golang.betsys.com --repo github.com/tomasji/webid-operator
```

### Create a new API and Controller
```
operator-sdk create api --group webid --version v1alpha1 --kind WebServer --resource --controller
```

- edit the generated API api/v1alpha1/webserver_types.go
- After modifying the `*_types.go` file, run: `make generate`
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
