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
	"github.com/alphayax/torrent-operator/internal/helper"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	torrentv1alpha1 "github.com/alphayax/torrent-operator/api/v1alpha1"
)

const BTSERVER_FINALIZER = "bt-server.bt.alphayax.com/finalizer"

// BtServerReconciler reconciles a BtServer object
type BtServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=torrent.alphayax.com,resources=btservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=torrent.alphayax.com,resources=btservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=torrent.alphayax.com,resources=btservers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BtServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *BtServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	k8sBittorrent := &torrentv1alpha1.BtServer{}
	if err := r.Get(ctx, req.NamespacedName, k8sBittorrent); err != nil {
		if errors.IsNotFound(err) {
			// Server has been deleted
			// TODO: Remove torrent linked to this server (without interacting with torrent API)
			return ctrl.Result{}, nil
		}
		// Unknown error (We don't go further)
		return ctrl.Result{}, err
	}

	// Handle Finalizer
	if k8sBittorrent.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(k8sBittorrent, BTSERVER_FINALIZER) {
			controllerutil.AddFinalizer(k8sBittorrent, BTSERVER_FINALIZER)
			if err := r.Update(ctx, k8sBittorrent); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(k8sBittorrent, BTSERVER_FINALIZER) {
			if err := r.deleteServer(ctx, k8sBittorrent); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(k8sBittorrent, BTSERVER_FINALIZER)
			if err := r.Update(ctx, k8sBittorrent); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Register server if needed
	btClient, err := getBtServer(ctx, k8sBittorrent)
	if err != nil {
		k8sBittorrent.Status.State = "Not connected"
		k8sBittorrent.Status.ServerVersion = "Unknown"
		if err := r.Status().Update(ctx, k8sBittorrent); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, err
	}

	version, err := btClient.GetAPIVersion()
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update state
	k8sBittorrent.Status.State = "Connected"
	k8sBittorrent.Status.ServerVersion = version
	if err := r.Status().Update(ctx, k8sBittorrent); err != nil {
		return ctrl.Result{}, err
	}

	kubeTorrents, err := r.getKubeTorrents(ctx, k8sBittorrent)
	if err != nil {
		return ctrl.Result{}, err
	}
	serverTorrents, err := r.getServerTorrents(ctx, btClient)
	if err != nil {
		return ctrl.Result{}, err
	}

	for hash, torrent := range serverTorrents {
		// Torrent exist on server but is not referenced in kube
		if _, ok := kubeTorrents[hash]; !ok {
			if err := r.CreateTorrent(ctx, req, torrent); err != nil {
				logger.Error(err, "Error while torrent creation")
			}
		}
	}

	for hash, kubeTorrent := range kubeTorrents {
		if kubeTorrent.Spec.Hash == "" {
			// Pending torrent creation
			continue
		}
		// Torrent is in kube (and initialized), but not on server
		if _, ok := serverTorrents[hash]; !ok {
			if err := r.DeleteTorrent(ctx, kubeTorrent); err != nil {
				logger.Error(err, "Error while torrent delete")
			}
		}
	}

	return ctrl.Result{
		RequeueAfter: 1 * time.Minute,
	}, nil
}

func (r *BtServerReconciler) getServerTorrents(ctx context.Context, qb helper.BtServerInterface) (map[string]*helper.Torrent, error) {
	torrentList, err := qb.GetTorrents()
	if err != nil {
		return nil, err
	}

	serverTorrents := make(map[string]*helper.Torrent, len(torrentList))
	for _, torrent := range torrentList {
		serverTorrents[torrent.Hash] = torrent
	}

	return serverTorrents, nil
}

func (r *BtServerReconciler) getKubeTorrents(ctx context.Context, btServer *torrentv1alpha1.BtServer) (map[string]torrentv1alpha1.Torrent, error) {
	torrentList := torrentv1alpha1.TorrentList{}
	if err := r.List(ctx, &torrentList, &client.ListOptions{
		Namespace:     btServer.Namespace,
		FieldSelector: fields.OneTermEqualSelector("spec.serverRef.name", btServer.Name),
	}); err != nil {
		if errors.IsNotFound(err) {
			// No torrent found
			return nil, nil
		}
		// Unknown error (We don't go further)
		return nil, err
	}

	kubeTorrents := make(map[string]torrentv1alpha1.Torrent)
	for _, torrent := range torrentList.Items {
		kubeTorrents[torrent.Spec.Hash] = torrent
	}

	return kubeTorrents, nil
}

func (r *BtServerReconciler) CreateTorrent(ctx context.Context, req ctrl.Request, torrent *helper.Torrent) error {
	logger := log.FromContext(ctx)
	logger.Info("* Upsert torrent", "hash", torrent.Hash)

	// Try to find if torrent is already in kube but not have a hash already
	// If yes, update it
	torrentName := torrent.Hash
	if torrent.Tags != "" {
		TagList := strings.Split(torrent.Tags, ",")
		for _, tag := range TagList {
			if strings.HasPrefix(tag, "k8s-") {
				torrentName = strings.TrimPrefix(tag, "k8s-")
			}
		}
	}

	// If a torrentName is found, it's a reference to a k8s object. Try to get it
	k8sTorrent := &torrentv1alpha1.Torrent{}
	namespacedName := client.ObjectKey{
		Namespace: req.Namespace,
		Name:      torrentName,
	}

	err := r.Get(ctx, namespacedName, k8sTorrent)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	torrentFound := !errors.IsNotFound(err)

	// Add Torrent metadata
	k8sTorrent.Name = torrentName
	k8sTorrent.Namespace = req.Namespace

	// Update Torrent Specs
	k8sTorrent.Spec.Hash = torrent.Hash
	k8sTorrent.Spec.Name = torrent.Name
	k8sTorrent.Spec.DownloadDir = torrent.DownloadDir
	k8sTorrent.Spec.ServerRef = torrentv1alpha1.ServerRef{
		Namespace: req.Namespace,
		Name:      req.Name,
	}

	// Set Spec default values according origin
	if torrentFound {
		k8sTorrent.Spec.ManagedBy = "k8s"
	} else {
		k8sTorrent.Spec.ManagedBy = "btServer"
		k8sTorrent.Spec.KeepFiles = true
	}

	// Create/Update the torrent
	if !torrentFound {
		return r.Create(ctx, k8sTorrent)
	}
	return r.Update(ctx, k8sTorrent)
}

func (r *BtServerReconciler) DeleteTorrent(ctx context.Context, torrent torrentv1alpha1.Torrent) error {
	logger := log.FromContext(ctx)

	// If torrent is managed by server, delete it from kube
	if torrent.Spec.ManagedBy != "k8s" {
		logger.Info("Delete torrent", "name", torrent.Name, "hash", torrent.Spec.Hash)
		return r.Delete(ctx, &torrent)
	}

	// Torrent should be here. Maybe stuck by finalizer ?
	// TODO: Wait the case appears and decide what to do...
	logger.Info("!!!! Torrent is managed by k8s, but is not on server", "name", torrent.Name, "hash", torrent.Spec.Hash)
	logger.Info("!!!! Should we recreate it ?")
	return nil
}

func (r *BtServerReconciler) deleteServer(ctx context.Context, qBittorrent *torrentv1alpha1.BtServer) error {
	logger := log.FromContext(ctx)

	// Get all torrent objects linked to this server
	kubeTorrents, err := r.getKubeTorrents(ctx, qBittorrent)
	if err != nil {
		return err
	}

	for _, torrent := range kubeTorrents {
		logger.Info("Remove orphan torrent", "name", torrent.Name, "hash", torrent.Spec.Hash)
		if err := r.Delete(ctx, &torrent); err != nil {
			logger.Error(err, "Unable to remove orphan torrent")
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BtServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&torrentv1alpha1.BtServer{}).
		Complete(r)
}
