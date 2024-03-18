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
	torrentv1alpha1 "github.com/alphayax/torrent-operator/api/v1alpha1"
	"github.com/alphayax/torrent-operator/internal/helper"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"time"
)

const TORRENT_FINALIZER = "torrent.bt.alphayax.com/finalizer"

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
	//logger.Info("*** RECONCILE torrent ***")

	// Retrieve torrent object
	torrent := torrentv1alpha1.Torrent{}
	if err := r.Get(ctx, req.NamespacedName, &torrent); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle Finalizer
	if torrent.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&torrent, TORRENT_FINALIZER) {
			controllerutil.AddFinalizer(&torrent, TORRENT_FINALIZER)
			if err := r.Update(ctx, &torrent); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(&torrent, TORRENT_FINALIZER) {
			if err := r.deleteTorrent(ctx, &torrent); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(&torrent, TORRENT_FINALIZER)
			if err := r.Update(ctx, &torrent); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Connect to server
	btServer, err := r.connectToServer(ctx, torrent.Spec.ServerRef)
	if err != nil {
		return ctrl.Result{}, err
	}

	// In case of new torrent (Hash is empty)
	if torrent.Spec.Hash == "" && torrent.Spec.URL != "" {
		logger.Info("--- Add Torrent (via URL)", "URL", torrent.Spec.URL)

		if err := btServer.AddTorrentByURL(ctx, torrent.Spec.URL, &torrent); err != nil {
			logger.Error(err, "Unable to add torrent")
		}

		// Stop here.
		// TODO: Trigger server for resync and hash update
		return ctrl.Result{
			RequeueAfter: 2 * time.Minute,
		}, err
	}

	// Refresh current torrent state
	torrentInit, err := btServer.GetTorrent(ctx, torrent.Spec.Hash)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Pause/Resume Torrent
	if torrent.Spec.Paused == true && !torrentInit.IsPaused {
		if err := btServer.StopTorrent(ctx, torrent.Spec.Hash); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("--- Torrent STOPPED", "Hash", torrent.Spec.Hash)
	}
	if torrent.Spec.Paused == false && torrentInit.IsPaused {
		if err := btServer.ResumeTorrent(ctx, torrent.Spec.Hash); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("--- Resume RESUMED", "Hash", torrent.Spec.Hash)
	}

	// Update Name
	if torrentInit.Name != torrent.Spec.Name {
		if err := btServer.RenameTorrent(ctx, torrent.Spec.Hash, torrent.Spec.Name); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("--- Torrent RENAMED", "Hash", torrent.Spec.Hash, "NewName", torrent.Spec.Name)
	}

	// Update Folder
	if torrentInit.DownloadDir != torrent.Spec.DownloadDir {
		if err := btServer.MoveTorrent(ctx, torrent.Spec.Hash, torrent.Spec.DownloadDir); err != nil {
			return ctrl.Result{}, err
		}
		logger.Info("--- Torrent MOVED", "Hash", torrent.Spec.Hash, "NewFolder", torrent.Spec.DownloadDir)
	}

	// Update State
	torrentStatus, err := btServer.GetTorrentStatus(ctx, torrent.Spec.Hash)
	if err != nil {
		return ctrl.Result{}, err
	}

	torrent.Status = torrentStatus
	return ctrl.Result{
		RequeueAfter: 1 * time.Minute,
	}, r.Status().Update(ctx, &torrent)
}

func (r *TorrentReconciler) deleteTorrent(ctx context.Context, torrent *torrentv1alpha1.Torrent) error {
	logger := log.FromContext(ctx)
	logger.Info("Delete torrent", "Name", torrent.Name)

	// If torrent is not managed by this controller, we don't have anything to do
	if torrent.Spec.ManagedBy != "k8s" {
		return nil
	}

	// Connect to server
	btServer, err := r.connectToServer(ctx, torrent.Spec.ServerRef)
	if err != nil {
		return err
	}

	logger.Info("Delete torrent", "Name", torrent.Name, "KeepFiles", torrent.Spec.KeepFiles)
	return btServer.DeleteTorrent(ctx, torrent.Spec.Hash, torrent.Spec.KeepFiles)
}

func (r *TorrentReconciler) connectToServer(ctx context.Context, ref torrentv1alpha1.ServerRef) (helper.BtServerInterface, error) {
	k8sBtServer := &torrentv1alpha1.BtServer{}
	if err := r.Get(ctx, ref.GetNamespacedName(), k8sBtServer); err != nil {
		return nil, err
	}

	return getBtServer(ctx, k8sBtServer)
}

// SetupWithManager sets up the controller with the Manager.
func (r *TorrentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Index Torrents by ServerRef
	if err := mgr.GetCache().IndexField(
		context.Background(),
		&torrentv1alpha1.Torrent{},
		"spec.serverRef.name",
		func(obj client.Object) []string {
			return []string{obj.(*torrentv1alpha1.Torrent).Spec.ServerRef.Name}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&torrentv1alpha1.Torrent{}).
		Complete(r)
}
