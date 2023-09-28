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

type V2rayUserInfo struct {
	// +kubebuilder:validation:MinLength=36
	// +kubebuilder:validation:MaxLength=36
	// +kubebuilder:validation:Pattern="^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"
	//	uuid field, if empty, will be generated automatically
	Id string `json:"id,omitempty"`

	Level int `json:"level,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=8
	// +kubebuilder:validation:MaxLength=32
	Email string `json:"email,omitempty"`
}

type V2rayNodeInfo struct {
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`
	// +optional
	//	Fetch from node info if not provided
	Ip string `json:"ip,omitempty"`
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:validation:ExclusiveMaximum=false
	// +kubebuilder:default=443
	Port int32 `json:"port,omitempty"`
	// +optional
	// +kubebuilder:default="external.hdcjh.xyz"
	Domain string `json:"domain,omitempty"`

	// +optional
	//	filled by v2ray-operator
	SvcName string `json:"svcName,omitempty"`
}

type V2rayNodeList struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	Size int32 `json:"size,omitempty"`
	// +kubebuilder:default=false
	// whether the node ip only owned by this v2ray client
	VirginIp bool `json:"virginIp,omitempty"`

	// +optional
	Info []V2rayNodeInfo `json:"info,omitempty"`
}

// V2rayUserSpec defines the desired state of V2rayUser
type V2rayUserSpec struct {
	// +kubebuilder:validation:Required
	User V2rayUserInfo `json:"user,omitempty"`

	// +kubebuilder:validation:Required
	NodeList map[string]*V2rayNodeList `json:"nodeList"`

	// +optional
	// +kubebuilder:default="ws+vmess"
	// +kubebuilder:validation:Enum="ws+vmess";"ws+vless"
	Protocol string `json:"protocol,omitempty"`
}

// V2rayUserStatus defines the observed state of V2rayUser
type V2rayUserStatus struct {
	// Represents the observations of a V2rayUser's current state.
	// V2rayUser.status.conditions.type are: "Available", "Progressing", and "Degraded"
	// V2rayUser.status.conditions.status are one of True, False, Unknown.
	// V2rayUser.status.conditions.reason the value should be a CamelCase string and producers of specific
	// condition types may define expected values and meanings for this field, and whether the values
	// are considered a guaranteed API.
	// V2rayUser.status.conditions.Message is a human readable message indicating details about the transition.
	// For further information see: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// V2rayUser is the Schema for the v2rayusers API
type V2rayUser struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   V2rayUserSpec   `json:"spec,omitempty"`
	Status V2rayUserStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// V2rayUserList contains a list of V2rayUser
type V2rayUserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []V2rayUser `json:"items"`
}

func init() {
	SchemeBuilder.Register(&V2rayUser{}, &V2rayUserList{})
}
