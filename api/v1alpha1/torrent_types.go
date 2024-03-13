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

// TorrentSpec defines the DESIRED state of Torrent
type TorrentSpec struct {
	//+kubebuilder:validation:Required
	ServerRef ServerRef `json:"serverRef"`

	// Hash represent the torrent Hash. It is used to identify the torrent and set by the Bittorrent Server.
	Hash string `json:"hash,omitempty"`

	// Name of the torrent
	//+kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`

	//+kubebuilder:validation:Optional
	URL string `json:"url,omitempty"`

	//TorrentFile 	   string `json:"torrentFile,omitempty"`

	// Paused will add the torrent in the paused state
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=false
	Paused bool `json:"paused"`

	// KeepFiles will keep the downloaded files after the torrent is removed
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=true
	KeepFiles bool `json:"keepFiles"`

	// ManagedBy define if the torrent is managed by kubernetes or not
	//+kubebuilder:validation:Optional
	//+kubebuilder:default:=k8s
	ManagedBy string `json:"managedBy,omitempty"`

	// DownloadDir define the folder where the files will be downloaded
	//+kubebuilder:validation:Optional
	DownloadDir string `json:"downloadDir,omitempty"`
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

// TorrentStatus defines the OBSERVED state of Torrent
type TorrentStatus struct {
	AddedOn  string             `json:"addedOn"`
	State    string             `json:"state,omitempty"`
	Ratio    string             `json:"ratio,omitempty"`
	Progress string             `json:"progress,omitempty"`
	Size     string             `json:"size"`
	Eta      string             `json:"eta"`
	Speed    TorrentStatusSpeed `json:"speed"`
	Peers    TorrentStatusPeers `json:"peers"`
	Data     TorrentStatusData  `json:"data"`
}

type TorrentStatusData struct {
	Downloaded string `json:"downloaded"`
	Uploaded   string `json:"uploaded"`
}
type TorrentStatusPeers struct {
	Seeders  string `json:"seeders"`
	Leechers string `json:"leechers"`
}

type TorrentStatusSpeed struct {
	DlSpeed int `json:"dlSpeed"`
	UpSpeed int `json:"upSpeed"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Torrent is the Schema for the torrents API
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.state`
// +kubebuilder:printcolumn:name="Ratio",type=string,JSONPath=`.status.ratio`
// +kubebuilder:printcolumn:name="Seed",type=string,JSONPath=`.status.peers.seeders`
// +kubebuilder:printcolumn:name="Leech",type=string,JSONPath=`.status.peers.leechers`
// +kubebuilder:printcolumn:name="Content",type=string,JSONPath=`.spec.name`
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
