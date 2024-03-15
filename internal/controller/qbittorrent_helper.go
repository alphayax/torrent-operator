package controller

import (
	"context"
	"fmt"
	qbt "github.com/KnutZuidema/go-qbittorrent"
	torrentv1alpha1 "github.com/alphayax/torrent-operator/api/v1alpha1"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
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

		username, err := getUsername(ctx, qBittorrent.Spec.Credentials)
		if err != nil {
			return err
		}
		password, err := getPassword(ctx, qBittorrent.Spec.Credentials)
		if err != nil {
			return err
		}

		qbtServers[serverKey] = qbt.NewClient(qBittorrent.Spec.ServerUri, logrus.New())
		if err := qbtServers[serverKey].Login(username, password); err != nil {
			return err
		}
		logger.Info("Logged into server", "Uri", qBittorrent.Spec.ServerUri)
	}

	return nil
}

func getServerKey(qBittorrent *torrentv1alpha1.QBittorrentServer) string {
	return fmt.Sprintf("%s/%s", qBittorrent.Namespace, qBittorrent.Name)
}

func getServer(ctx context.Context, btServer *torrentv1alpha1.QBittorrentServer) (*qbt.Client, error) {
	serverKey := getServerKey(btServer)

	// If server is not found, try to register it
	if _, ok := qbtServers[serverKey]; !ok {
		if err := registerServer(ctx, btServer); err != nil {
			return nil, err
		}
	}

	return qbtServers[serverKey], nil
}

func getUsername(ctx context.Context, credentials torrentv1alpha1.QBittorrentServerSpecCredentials) (string, error) {
	// Get username from secret if specified
	if credentials.UsernameFromSecret.Name != "" {
		return getValueFromSecret(
			ctx,
			credentials.UsernameFromSecret.Name,
			credentials.UsernameFromSecret.Namespace,
			credentials.UsernameFromSecret.Key,
		)
	}
	// If not specified via a secret, return value in spec
	return credentials.Username, nil
}

func getPassword(ctx context.Context, credentials torrentv1alpha1.QBittorrentServerSpecCredentials) (string, error) {
	// Get password from secret if specified
	if credentials.PasswordFromSecret.Name != "" {
		return getValueFromSecret(
			ctx,
			credentials.PasswordFromSecret.Name,
			credentials.PasswordFromSecret.Namespace,
			credentials.PasswordFromSecret.Key,
		)
	}
	// If not specified via a secret, return value in spec
	return credentials.Password, nil
}

func getValueFromSecret(ctx context.Context, secretName string, secretNamespace string, secretKey string) (string, error) {
	cl, err := client.New(config.GetConfigOrDie(), client.Options{})
	if err != nil {
		return "", err
	}

	namespacedName := types.NamespacedName{
		Name:      secretName,
		Namespace: secretNamespace,
	}
	secret := &v1.Secret{}
	err = cl.Get(ctx, namespacedName, secret)
	if err != nil {
		return "", err
	}

	return string(secret.Data[secretKey]), nil
}
