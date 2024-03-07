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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	torrentv1alpha1 "github.com/alphayax/torrent-operator/api/v1alpha1"
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
			// Todo: If server exists, delete the torrent from the server
			logger.Info("--- Torrent deleted")
			return ctrl.Result{}, nil
		}
		// Unknown error
		return ctrl.Result{}, err
	}

	if torrent.Spec.URL != "" {
		logger.Info("--- Add Torrent (via URL)", "URL", torrent.Spec.URL)
		err := r.addTorrentToServer(ctx, req, torrent.Spec)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *TorrentReconciler) addTorrentToServer(ctx context.Context, req ctrl.Request, spec torrentv1alpha1.TorrentSpec) error {

	// TODO: Handle case where server is not qBittorrent

	qBittorrent := torrentv1alpha1.QBittorrentServer{}
	if err := r.Get(ctx, spec.ServerRef.GetNamespacedName(), &qBittorrent); err != nil {
		if errors.IsNotFound(err) {
			// Server has been deleted
			return fmt.Errorf("unable to find server %s/%s", spec.ServerRef.Namespace, spec.ServerRef.Name)
		}
		// Unknown error
		return err
	}

	// Server Exists
	logger := log.FromContext(ctx)
	logger.Info("** Adding torrent to server", "Server", qBittorrent.Name, "Torrent", req.Name)

	qBittorrent.Status.PendingTorrents = append(qBittorrent.Status.PendingTorrents, req.Name)
	if err := r.Status().Update(ctx, &qBittorrent); err != nil {
		return err
	}

	anno := qBittorrent.GetAnnotations()
	anno["pendingTorrents"] = "true"
	qBittorrent.SetAnnotations(anno)
	return r.Client.Update(ctx, &qBittorrent)

	//return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *TorrentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&torrentv1alpha1.Torrent{}).
		Complete(r)
}
