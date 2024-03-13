package controller

import (
	"context"
	"crypto/sha256"
	"fmt"
	qbt "github.com/KnutZuidema/go-qbittorrent"
	torrentv1alpha1 "github.com/alphayax/torrent-operator/api/v1alpha1"
	"github.com/sirupsen/logrus"
	"sync"
)

var (
	qbtServers      = map[string]*qbt.Client{}
	qbtServersMutex sync.Mutex
)

func connectToServer(ctx context.Context, qBittorrent torrentv1alpha1.QBittorrentServer) (*qbt.Client, error) {
	var err error

	if qbtServers == nil {
		qbtServers = map[string]*qbt.Client{}
	}

	// Avoid creation of multiple clients for the same server
	qbtServersMutex.Lock()
	defer qbtServersMutex.Unlock()

	// Check if we already have a client for this server
	serverKey := fmt.Sprintf("%x", sha256.Sum256([]byte(qBittorrent.Spec.ServerUri)))
	if _, ok := qbtServers[serverKey]; !ok {
		qbtServers[serverKey] = qbt.NewClient(qBittorrent.Spec.ServerUri, logrus.New())
		err = qbtServers[serverKey].Login(
			qBittorrent.Spec.Credentials.Username,
			qBittorrent.Spec.Credentials.Password,
		)
	}

	return qbtServers[serverKey], err
}
