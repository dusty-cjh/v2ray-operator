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
	"github.com/dusty-cjh/v2ray-operator/internal/managers/v2fly"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type V2rayRegionAffinity struct {
	// +kubebuilder:validation:Enum=us;sg;hk;jp;tw;au;uk;de;fr;ca;in;th;vn;my;ph;id;br;mx
	// +kubebuilder:default=us
	//	which region the v2ray instance should belongs to
	Region string `json:"region,omitempty"`

	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=5
	// +kubebuilder:validation:ExclusiveMaximum=false
	// +kubebuilder:default=1
	//	how many v2ray instances in this region
	Nums int `json:"nums,omitempty"`

	// +kubebuilder:default=false
	// whether the node ip only owned by this v2ray client
	VirginIp bool `json:"virginIp,omitempty"`
}

type V2rayNodeAffinity struct {
	Regions []V2rayRegionAffinity `json:"regions,omitempty"`
}

// V2rayClientSpec defines the desired state of V2rayClient
type V2rayClientSpec struct {
	// +kubebuilder:validation:Required
	V2ray *v2fly.V2rayInstanceConfig `json:"v2ray"`
	// +kubebuilder:validation:Required
	Affinity *V2rayNodeAffinity `json:"affinity,omitempty"`
}

// V2rayClientStatus defines the observed state of V2rayClient
type V2rayClientStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// +optional
	SubscriptionLink string `json:"subscriptionLink,omitempty" patchStrategy:"merge"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// V2rayClient is the Schema for the v2rayclients API
type V2rayClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   V2rayClientSpec   `json:"spec,omitempty"`
	Status V2rayClientStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// V2rayClientList contains a list of V2rayClient
type V2rayClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []V2rayClient `json:"items"`
}

func init() {
	SchemeBuilder.Register(&V2rayClient{}, &V2rayClientList{})
}
