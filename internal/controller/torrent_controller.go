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

package controller

import (
	"context"
	"crypto/sha256"
	"fmt"
	qbt "github.com/KnutZuidema/go-qbittorrent"
	"github.com/KnutZuidema/go-qbittorrent/pkg/model"
	torrentv1alpha1 "github.com/alphayax/torrent-operator/api/v1alpha1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

// TorrentReconciler reconciles a Torrent object
type TorrentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=torrent.alphayax.com,resources=torrents,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=torrent.alphayax.com,resources=torrents/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=torrent.alphayax.com,resources=torrents/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *TorrentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("*** RECONCILE torrent ***")

	// Retrieve torrent object
	torrent := torrentv1alpha1.Torrent{}
	if err := r.Get(ctx, req.NamespacedName, &torrent); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// TODO Define a default server if not defined
	// torrent.Spec.ServerRef

	// Connect to server
	qb, err := r.connectToServer(ctx, torrent.Spec.ServerRef)
	if err != nil {
		return ctrl.Result{}, err
	}

	// In case of new torrent (Hash is empty)
	if torrent.Spec.Hash == "" && torrent.Spec.URL != "" {
		logger.Info("--- Add Torrent (via URL)", "URL", torrent.Spec.URL)
		URLs := []string{torrent.Spec.URL}
		torrentOptions := &model.AddTorrentsOptions{
			Tags:     fmt.Sprintf("k8s-%s", req.Name),
			Savepath: torrent.Spec.DownloadDir,
			Rename:   torrent.Spec.Name,
			Paused:   strconv.FormatBool(torrent.Spec.Paused),
		}
		if err := qb.Torrent.AddURLs(URLs, torrentOptions); err != nil {
			logger.Error(err, "!!! Error while adding torrent")
		}

		// Stop here.
		// TODO: Trigger server for resync and hash update
		return ctrl.Result{
			RequeueAfter: 2 * time.Minute,
		}, err
	}

	// Refresh current torrent state
	list, err := qb.Torrent.GetList(&model.GetTorrentListOptions{Hashes: torrent.Spec.Hash})
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(list) == 0 {
		// TODO: Better handeling: We don't requeue here, but we should handle the thing better
		return ctrl.Result{}, nil
	}
	torrentInit := list[0]

	// Pause/Resume Torrent
	if torrent.Spec.Paused == true && (torrentInit.State != "pausedUP" && torrentInit.State != "pausedDL") {
		if err := qb.Torrent.StopTorrents([]string{torrent.Spec.Hash}); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("--- Torrent STOPPED", "Hash", torrent.Spec.Hash)
	} else if torrent.Spec.Paused == false && (torrentInit.State == "pausedUP" || torrentInit.State == "pausedDL") {
		if err := qb.Torrent.ResumeTorrents([]string{torrent.Spec.Hash}); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("--- Resume RESUMED", "Hash", torrent.Spec.Hash)
	}

	// Update Name
	if torrentInit.Name != torrent.Spec.Name {
		if err := qb.Torrent.SetName(torrent.Spec.Hash, torrent.Spec.Name); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("--- Torrent RENAMED", "Hash", torrent.Spec.Hash, "NewName", torrent.Spec.Name)
	}

	// Update Folder
	if torrentInit.SavePath != torrent.Spec.DownloadDir {
		if err := qb.Torrent.SetLocations([]string{torrent.Spec.Hash}, torrent.Spec.DownloadDir); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("--- Torrent MOVED", "Hash", torrent.Spec.Hash, "NewFolder", torrent.Spec.DownloadDir)
	}

	// Update State
	list, err = qb.Torrent.GetList(&model.GetTorrentListOptions{Hashes: torrent.Spec.Hash})
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(list) == 0 {
		// TODO: Better handeling: We don't requeue here, but we should handle the thing better
		return ctrl.Result{}, nil
	}

	return ctrl.Result{
		RequeueAfter: 1 * time.Minute,
	}, r.updateTorrentState(ctx, &torrent, list[0])
}

// updateTorrentState Update the torrent according to the server
func (r *TorrentReconciler) updateTorrentState(ctx context.Context, torrent *torrentv1alpha1.Torrent, qbTorrent *model.Torrent) error {
	torrent.Status.AddedOn = time.Unix(int64(qbTorrent.AddedOn), 0).Format(time.RFC3339)
	torrent.Status.State = string(qbTorrent.State)
	torrent.Status.Progress = fmt.Sprintf("%.2f%%", qbTorrent.Progress*100)
	torrent.Status.Ratio = fmt.Sprintf("%.3f", qbTorrent.Ratio)
	torrent.Status.Speed.DlSpeed = qbTorrent.Dlspeed
	torrent.Status.Speed.UpSpeed = qbTorrent.Upspeed
	torrent.Status.Eta = (time.Duration(qbTorrent.Eta) * time.Second).String()
	torrent.Status.Size = HumanReadableSize(int64(qbTorrent.Size))
	torrent.Status.Data.Downloaded = HumanReadableSize(qbTorrent.Downloaded)
	torrent.Status.Data.Uploaded = HumanReadableSize(qbTorrent.Uploaded)
	torrent.Status.Peers.Leechers = fmt.Sprintf("%d/%d", qbTorrent.NumLeechs, qbTorrent.NumIncomplete)
	torrent.Status.Peers.Seeders = fmt.Sprintf("%d/%d", qbTorrent.NumSeeds, qbTorrent.NumComplete)
	return r.Status().Update(ctx, torrent)
}

// HumanReadableSize Convert a size in bytes to a human readable size
// TODO: Export to another file
func HumanReadableSize(sizeInit int64) string {
	// Handle zero size as a special case
	if sizeInit == 0 {
		return "0 B"
	}

	units := []string{"B", "KB", "MB", "GB", "TB", "PB", "EB"}
	size := float64(sizeInit)

	// Use float division for more accurate calculations
	index := 0
	for size >= 1024 && index < len(units)-1 {
		size = float64(size) / 1024
		index++
	}

	// Round the value to one decimal place and format output
	return fmt.Sprintf("%.3f %s", size, units[index])
}

var qbtServers = map[string]*qbt.Client{}

func (r *TorrentReconciler) connectToServer(ctx context.Context, ref torrentv1alpha1.ServerRef) (*qbt.Client, error) {
	qBittorrent := &torrentv1alpha1.QBittorrentServer{}
	if err := r.Get(ctx, ref.GetNamespacedName(), qBittorrent); err != nil {
		if errors.IsNotFound(err) {
			return nil, err
		}
		return nil, err
	}

	// Check if we already have a client for this server
	var err error
	serverKey := fmt.Sprintf("%x", sha256.Sum256([]byte(qBittorrent.Spec.Server)))
	if _, ok := qbtServers[serverKey]; !ok {
		qbtServers[serverKey] = qbt.NewClient(qBittorrent.Spec.Server, logrus.New())
		err = qbtServers[serverKey].Login(qBittorrent.Spec.Username, qBittorrent.Spec.Password)
	}

	return qbtServers[serverKey], err
}

// SetupWithManager sets up the controller with the Manager.
func (r *TorrentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&torrentv1alpha1.Torrent{}).
		Complete(r)
}
