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
	qbt "github.com/KnutZuidema/go-qbittorrent"
	"github.com/KnutZuidema/go-qbittorrent/pkg/model"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	torrentv1alpha1 "github.com/alphayax/torrent-operator/api/v1alpha1"
)

// QBittorrentServerReconciler reconciles a QBittorrentServer object
type QBittorrentServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=torrent.alphayax.com,resources=qbittorrentservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=torrent.alphayax.com,resources=qbittorrentservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=torrent.alphayax.com,resources=qbittorrentservers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the QBittorrentServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *QBittorrentServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("*** RECONCILE qBittorrent ***")

	qBittorrent := torrentv1alpha1.QBittorrentServer{}
	if err := r.Get(ctx, req.NamespacedName, &qBittorrent); err != nil {
		if errors.IsNotFound(err) {
			// Server has been deleted
			// TODO: Remove torrent linked to this server (without interacting with torrent API)
			return ctrl.Result{}, nil
		}
		// Unknown error (We don't go further)
		return ctrl.Result{}, err
	}

	// Server has been created
	qBittorrentUrl := qBittorrent.Spec.Server
	qb := qbt.NewClient(qBittorrentUrl, logrus.New())
	err := qb.Login(qBittorrent.Spec.Username, qBittorrent.Spec.Password)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update state
	qBittorrent.Status.State = "Connected"
	if err := r.Status().Update(ctx, &qBittorrent); err != nil {
		return ctrl.Result{}, err
	}

	torrents, err := qb.Torrent.GetList(nil)
	if err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("* Refresh torrents from server")
	for _, torrent := range torrents {
		if err := r.UpsertTorrent(ctx, req, torrent); err != nil {
			logger.Error(err, "Error while torrent upsert")
		}
	}

	// TODO: Check all torrents and try to find the one that are not in the list anymore (deleted manualy form server)
	// Check for pending torrents
	if qBittorrent.GetAnnotations()["pendingTorrents"] != "" {
		logger.Info("--- Pending torrents found")
		for _, name := range qBittorrent.Status.PendingTorrents {
			logger.Info("--- Adding pending torrent", "Name", name)
			torrent := torrentv1alpha1.Torrent{}
			if err := r.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: name}, &torrent); err != nil {
				if errors.IsNotFound(err) {
					// Torrent not found ?!?
					logger.Error(err, "!!! Torrent not found", "Name", name)
					continue
				}
				// Unknown error
				logger.Error(err, "!!! Error while getting torrent", "Name", name)
				continue
			}

			if torrent.Spec.URL == "" {
				// Magnet Not found
				logger.Error(err, "!!! Magnet not found", "Name", name)
				continue
			} else {
				logger.Info("--- Magnet found", "URL", torrent.Spec.URL)
			}

			err := qb.Torrent.AddURLs([]string{torrent.Spec.URL}, nil)
			if err != nil {
				logger.Error(err, "!!! Error while adding torrent", "Name", name)
				continue
			}

			if err := r.Delete(ctx, &torrent); err != nil {
				logger.Error(err, "!!! Error while deleting torrent", "Name", name)
				continue
			}
		}

		qBittorrent.Status.PendingTorrents = nil
		if err := r.Status().Update(ctx, &qBittorrent); err != nil {
			logger.Error(err, "!!! Error while updating qBittorrent status")
		}

		anno := qBittorrent.GetAnnotations()
		delete(anno, "pendingTorrents")
		qBittorrent.SetAnnotations(anno)
		err := r.Client.Update(ctx, &qBittorrent)
		if err != nil {
			logger.Error(err, "!!! Error while updating qBittorrent annotations")
		}
	} else {
		logger.Info("--- NO Pending torrents found")
	}

	// Handle Pause and UnPause
	//qb.Torrent.ResumeTorrents(hash)
	//qb.Torrent.StopTorrents(hash)

	logger.Info("--- Requeue after 1 minutes")
	return ctrl.Result{
		RequeueAfter: 1 * time.Minute,
	}, nil
}

func (r *QBittorrentServerReconciler) UpsertTorrent(ctx context.Context, req ctrl.Request, qbTorrent *model.Torrent) error {
	logger := log.FromContext(ctx)
	torrent := torrentv1alpha1.Torrent{}

	namespacedName := client.ObjectKey{
		Namespace: req.Namespace,
		Name:      qbTorrent.Hash,
	}

	err := r.Get(ctx, namespacedName, &torrent)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	torrent.Name = qbTorrent.Hash
	torrent.Namespace = req.Namespace

	// TODO: WARN ! We should update STATE not SPEC (except for creation)
	// Update torrent spec
	torrent.Spec.Hash = qbTorrent.Hash
	torrent.Spec.Name = qbTorrent.Name // Should be in state ? we may rename the torrent

	torrent.Spec.ServerRef = torrentv1alpha1.ServerRef{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	if errors.IsNotFound(err) {
		// Create the torrent
		logger.Info("* Create torrent")
		if err := r.Create(ctx, &torrent); err != nil {
			return err
		}
	} else {
		// Update the torrent
		if err := r.Update(ctx, &torrent); err != nil {
			return err
		}
	}

	// Todo: Get all the torrent with the same serverRef and check if they are still in the list

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *QBittorrentServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&torrentv1alpha1.QBittorrentServer{}).
		Complete(r)
}
