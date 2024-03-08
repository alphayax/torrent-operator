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

	torrent := torrentv1alpha1.Torrent{}
	if err := r.Get(ctx, req.NamespacedName, &torrent); err != nil {
		if errors.IsNotFound(err) {
			// Torrent has been deleted
			// TODO: Remove torrent from server
			return ctrl.Result{}, nil
		}
		// Unknown error
		return ctrl.Result{}, err
	}

	// Connect to server
	qb, err := r.connectToServer(ctx, torrent.Spec.ServerRef)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO: Use a specific TorrentRequest object ?
	if torrent.Spec.URL != "" {
		logger.Info("--- Add Torrent (via URL)", "URL", torrent.Spec.URL)
		err := r.addTorrentToServer(ctx, req, torrent.Spec)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Pause/Resume Torrent
	if torrent.Spec.Paused == true && (torrent.Status.State != "pausedUP" && torrent.Status.State != "pausedDL") {
		logger.Info("--- Pause Torrent", "Hash", torrent.Spec.Hash)
		if err := qb.Torrent.StopTorrents([]string{torrent.Spec.Hash}); err != nil {
			return ctrl.Result{}, err
		} else {
			logger.Info("--OK-- Pause torrent")
		}
	} else if torrent.Spec.Paused == false && (torrent.Status.State == "pausedUP" || torrent.Status.State == "pausedDL") {
		logger.Info("--- Resume Torrent")
		if err := qb.Torrent.ResumeTorrents([]string{torrent.Spec.Hash}); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Update State
	list, err := qb.Torrent.GetList(&model.GetTorrentListOptions{Hashes: torrent.Spec.Hash})
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{
		RequeueAfter: 1 * time.Minute,
	}, r.updateTorrentState(ctx, &torrent, list[0])
}

// updateTorrentState Update the torrent according to the server
func (r *TorrentReconciler) updateTorrentState(ctx context.Context, torrent *torrentv1alpha1.Torrent, qbTorrent *model.Torrent) error {
	torrent.Status.State = string(qbTorrent.State)
	torrent.Status.Progress = fmt.Sprintf("%.2f%%", qbTorrent.Progress*100)
	torrent.Status.Ratio = fmt.Sprintf("%.3f", qbTorrent.Ratio)
	torrent.Status.Speed.DlSpeed = qbTorrent.Dlspeed
	torrent.Status.Speed.UpSpeed = qbTorrent.Upspeed
	torrent.Status.Eta = (time.Duration(qbTorrent.Eta) * time.Second).String()
	torrent.Status.Size = HumanReadableSize(int64(qbTorrent.Size))
	torrent.Status.Downloaded = HumanReadableSize(qbTorrent.Downloaded)
	torrent.Status.Uploaded = HumanReadableSize(qbTorrent.Uploaded)
	torrent.Status.Peers.Leechers = fmt.Sprintf("%d/%d", qbTorrent.NumLeechs, qbTorrent.NumIncomplete)
	torrent.Status.Peers.Seeders = fmt.Sprintf("%d/%d", qbTorrent.NumSeeds, qbTorrent.NumComplete)
	return r.Status().Update(ctx, torrent)
}

func HumanReadableSize(size int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	unit := 0
	for size > 1024 {
		size = size / 1024
		unit++
	}
	return fmt.Sprintf("%d %s", size, units[unit])
}

func (r *TorrentReconciler) connectToServer(ctx context.Context, ref torrentv1alpha1.ServerRef) (*qbt.Client, error) {
	qBittorrent := &torrentv1alpha1.QBittorrentServer{}
	if err := r.Get(ctx, ref.GetNamespacedName(), qBittorrent); err != nil {
		if errors.IsNotFound(err) {
			return nil, err
		}
		return nil, err
	}

	qb := qbt.NewClient(qBittorrent.Spec.Server, logrus.New())
	err := qb.Login(qBittorrent.Spec.Username, qBittorrent.Spec.Password)

	return qb, err
}

func (r *TorrentReconciler) getServer(ctx context.Context, spec torrentv1alpha1.TorrentSpec) (*torrentv1alpha1.QBittorrentServer, error) {
	// TODO: Handle multiple type of servers
	qBittorrent := &torrentv1alpha1.QBittorrentServer{}
	if err := r.Get(ctx, spec.ServerRef.GetNamespacedName(), qBittorrent); err != nil {
		return nil, err
	}
	return qBittorrent, nil
}

func (r *TorrentReconciler) addTorrentToServer(ctx context.Context, req ctrl.Request, spec torrentv1alpha1.TorrentSpec) error {

	// TODO: Handle case where server is not qBittorrent
	qBittorrent, err := r.getServer(ctx, spec)
	if err != nil {
		return err
	}

	// Server Exists
	logger := log.FromContext(ctx)
	logger.Info("** Adding torrent to server", "Server", qBittorrent.Name, "Torrent", req.Name)

	qBittorrent.Status.PendingTorrents = append(qBittorrent.Status.PendingTorrents, req.Name)
	if err := r.Status().Update(ctx, qBittorrent); err != nil {
		return err
	}

	// TODO: Think about using a configmap or else for orders to server
	anno := qBittorrent.GetAnnotations()
	anno["pendingTorrents"] = "true"
	qBittorrent.SetAnnotations(anno)
	return r.Client.Update(ctx, qBittorrent)

	//return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TorrentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&torrentv1alpha1.Torrent{}).
		Complete(r)
}
