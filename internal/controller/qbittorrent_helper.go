package controller

import (
	"context"
	"fmt"
	qbt "github.com/KnutZuidema/go-qbittorrent"
	torrentv1alpha1 "github.com/alphayax/torrent-operator/api/v1alpha1"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
)

var (
	qbtServers      = map[string]*qbt.Client{}
	qbtServersMutex sync.Mutex
)

func registerServer(ctx context.Context, qBittorrent *torrentv1alpha1.QBittorrentServer) error {
	logger := log.FromContext(ctx)

	if qbtServers == nil {
		qbtServers = map[string]*qbt.Client{}
	}

	serverKey := getServerKey(qBittorrent)

	// Avoid creation of multiple clients for the same server
	qbtServersMutex.Lock()
	defer qbtServersMutex.Unlock()

	// Check if we already have a client for this server
	if _, ok := qbtServers[serverKey]; !ok {
		logger.Info("Register server", "server", serverKey)
		qbtServers[serverKey] = qbt.NewClient(qBittorrent.Spec.ServerUri, logrus.New())
		err := qbtServers[serverKey].Login(
			qBittorrent.Spec.Credentials.Username,
			qBittorrent.Spec.Credentials.Password,
		)
		logger.Info("Logged into server", "Uri", qBittorrent.Spec.ServerUri)
		return err
	}

	return nil
}

func getServerKey(qBittorrent *torrentv1alpha1.QBittorrentServer) string {
	return fmt.Sprintf("%s/%s", qBittorrent.Namespace, qBittorrent.Name)
}

func getServer(ctx context.Context, qBittorrent *torrentv1alpha1.QBittorrentServer) (*qbt.Client, error) {
	serverKey := getServerKey(qBittorrent)

	// If server is not found, try to register it
	if _, ok := qbtServers[serverKey]; !ok {
		if err := registerServer(ctx, qBittorrent); err != nil {
			return nil, err
		}
	}

	return qbtServers[serverKey], nil
}
