package controller

import (
	"context"
	"fmt"
	torrentv1alpha1 "github.com/alphayax/torrent-operator/api/v1alpha1"
	"github.com/alphayax/torrent-operator/internal/helper"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sync"
)

var (
	btServers      = map[string]helper.BtServerInterface{}
	btServersMutex sync.Mutex
)

func getServerKey(btServer *torrentv1alpha1.BtServer) string {
	return fmt.Sprintf("%s/%s", btServer.Namespace, btServer.Name)
}

func getBtServer(ctx context.Context, btServer *torrentv1alpha1.BtServer) (helper.BtServerInterface, error) {
	serverKey := getServerKey(btServer)

	// If server is not found, try to register it
	if _, ok := btServers[serverKey]; !ok {
		if err := registerServer(ctx, btServer); err != nil {
			return nil, err
		}
	}

	return btServers[serverKey], nil
}

func registerServer(ctx context.Context, k8sBtServer *torrentv1alpha1.BtServer) error {
	logger := log.FromContext(ctx)

	if btServers == nil {
		btServers = map[string]helper.BtServerInterface{}
	}

	serverKey := getServerKey(k8sBtServer)

	// Avoid creation of multiple clients for the same server
	btServersMutex.Lock()
	defer btServersMutex.Unlock()

	// Check if we already have a client for this server
	if _, ok := btServers[serverKey]; !ok {
		logger.Info("Register server", "server", serverKey)

		username, err := getUsername(ctx, k8sBtServer.Spec.Credentials)
		if err != nil {
			return err
		}
		password, err := getPassword(ctx, k8sBtServer.Spec.Credentials)
		if err != nil {
			return err
		}

		switch k8sBtServer.Spec.Type {
		case "qBittorrent":
			btServers[serverKey] = helper.NewQBittorrentServer(k8sBtServer.Spec)
			break
		case "transmission":
			btServers[serverKey] = helper.NewTransmissionServer(k8sBtServer.Spec)
			break
		default:
			return fmt.Errorf("unsupported server type: %s", k8sBtServer.Spec.Type)
		}

		// Login
		if err := btServers[serverKey].Login(username, password); err != nil {
			return err
		}
		logger.Info("Logged into server", "Uri", k8sBtServer.Spec.ServerUri)
	}

	return nil
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

func getUsername(ctx context.Context, credentials torrentv1alpha1.BtServerSpecCredentials) (string, error) {
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

func getPassword(ctx context.Context, credentials torrentv1alpha1.BtServerSpecCredentials) (string, error) {
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
