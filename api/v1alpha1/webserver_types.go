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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WebServerSpec defines the desired state of WebServer
type WebServerSpec struct {
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// Image defines the nginx docker image for the WebID server,
	// for example 'nginx:1.25.3'
	// +kubebuilder:example=nginx:1.25.3
	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Docker nginx image"
	Image string `json:"image,omitempty"`

	// Replicas defines the number of WebID instances
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Number of pods"
	Replicas int32 `json:"replicas,omitempty"`
}

// WebServerStatus defines the observed state of WebServer
type WebServerStatus struct {
	// Conditions store the status conditions of the Memcached instances
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// WebServer is the Schema for the webservers API
type WebServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WebServerSpec   `json:"spec,omitempty"`
	Status WebServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WebServerList contains a list of WebServer
type WebServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WebServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WebServer{}, &WebServerList{})
}
