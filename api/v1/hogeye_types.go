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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HogEyeSpec defines the desired state of HogEye
type HogEyeSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	AppTokenSecret string `json:"appTokenSecret,omitempty"`
	SlackChannels  string `json:"slackChannels,omitempty"`
	QueryNamespace string `json:"queryNamespace,omitempty"`
	QueryResources string `json:"queryResources,omitempty"`
	QueryTime      string `json:"queryTime,omitempty"`
	AgeThreshold   int    `json:"ageThreshold,omitempty"`
}

// HogEyeStatus defines the observed state of HogEye
type HogEyeStatus struct {
	Status string `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`,description="Will display one of [Watching, Redeploying, Terminating, Error]"
//+kubebuilder:printcolumn:name="Resource_Type",type=string,JSONPath=`.spec.queryResources`
//+kubebuilder:printcolumn:name="Observed_Namespace",type=string,JSONPath=`.spec.queryNamespace`
//+kubebuilder:printcolumn:name="Age_Threshold",type=string,JSONPath=`.spec.ageThreshold`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// HogEye is the Schema for the hogeyes API
type HogEye struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HogEyeSpec   `json:"spec,omitempty"`
	Status HogEyeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// HogEyeList contains a list of HogEye
type HogEyeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HogEye `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HogEye{}, &HogEyeList{})
}
