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
	"k8s.io/apimachinery/pkg/types"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TorrentSpec defines the desired state of Torrent
type TorrentSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	ServerRef ServerRef `json:"serverRef,omitempty"`
	Hash      string    `json:"hash,omitempty"`
	Name      string    `json:"name,omitempty"`
	Size      int       `json:"size,omitempty"`

	URL                 string `json:"url,omitempty"`
	Paused              bool   `json:"paused,omitempty"`
	DeleteFilesOnRemove bool   `json:"deleteFilesOnRemove,omitempty"`
	//TorrentFile 	   string `json:"torrentFile,omitempty"`
	//DownloadDir string    `json:"downloadDir,omitempty"`
}

type ServerRef struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

func (s *ServerRef) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: s.Namespace,
		Name:      s.Name,
	}
}

// TorrentStatus defines the observed state of Torrent
type TorrentStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	State    string `json:"state,omitempty"`
	Ratio    string `json:"ratio,omitempty"`
	Progress string `json:"progress,omitempty"`
	DlSpeed  int    `json:"dlSpeed,omitempty"`
	UpSpeed  int    `json:"upSpeed,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Torrent is the Schema for the torrents API
// +kubebuilder:printcolumn:name="Content",type=string,JSONPath=`.spec.name`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Ratio",type=string,JSONPath=`.status.ratio`
type Torrent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TorrentSpec   `json:"spec,omitempty"`
	Status TorrentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TorrentList contains a list of Torrent
type TorrentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Torrent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Torrent{}, &TorrentList{})
}
