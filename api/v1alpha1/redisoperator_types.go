/*
Copyright 2024 MatanMagen.

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

// RedisOperatorSpec defines the desired state of RedisOperator
type RedisOperatorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// redis version to deploy
	RedisVersion    string `json:"redisversion"`
	ExporterVersion string `json:"exporterVersion"`
	TeamName        string `json:"teamName"`
	Env             string `json:"env"`
}

// RedisOperatorStatus defines the observed state of RedisOperator
type RedisOperatorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// RedisOperator is the Schema for the redisoperators API
type RedisOperator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedisOperatorSpec   `json:"spec,omitempty"`
	Status RedisOperatorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RedisOperatorList contains a list of RedisOperator
type RedisOperatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RedisOperator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RedisOperator{}, &RedisOperatorList{})
}
