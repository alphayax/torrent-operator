/*
Copyright 2024.

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
// NOTE: json tags are required. Any new fields you add must have json tags for the fields to be serialized.
// Important: Run "make" to regenerate code after modifying this file

// QBittorrentServerSpec defines the DESIRED state of QBittorrentServer
type QBittorrentServerSpec struct {
	ServerUri   string                           `json:"serverUri"`
	Credentials QBittorrentServerSpecCredentials `json:"credentials,omitempty"`
}

type QBittorrentServerSpecCredentials struct {
	Username           string         `json:"username,omitempty"`
	Password           string         `json:"password,omitempty"`
	PasswordFromSecret ItemFromSecret `json:"passwordFromSecret,omitempty"` // TODO: Implement (not yet used)
	UsernameFromSecret ItemFromSecret `json:"UsernameFromSecret,omitempty"` // TODO: Implement (not yet used)
}

type ItemFromSecret struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
}

// QBittorrentServerStatus defines the OBSERVED state of QBittorrentServer
type QBittorrentServerStatus struct {
	State         string `json:"state,omitempty"`
	ServerVersion string `json:"serverVersion,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// QBittorrentServer is the Schema for the qbittorrentservers API
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Uri",type=string,JSONPath=`.spec.serverUri`
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.status.serverVersion`
type QBittorrentServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QBittorrentServerSpec   `json:"spec,omitempty"`
	Status QBittorrentServerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// QBittorrentServerList contains a list of QBittorrentServer
type QBittorrentServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QBittorrentServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&QBittorrentServer{}, &QBittorrentServerList{})
}
