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
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// QBittorrentServerSpec defines the desired state of QBittorrentServer
type QBittorrentServerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of QBittorrentServer. Edit qbittorrentserver_types.go to remove/update
	Server   string `json:"server,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// QBittorrentServerStatus defines the observed state of QBittorrentServer
type QBittorrentServerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	PendingTorrents []string `json:"pendingTorrents,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// QBittorrentServer is the Schema for the qbittorrentservers API
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
