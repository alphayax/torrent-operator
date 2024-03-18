package helper

import (
	"context"
	torrentv1alpha1 "github.com/alphayax/torrent-operator/api/v1alpha1"
)

type BtServerInterface interface {
	Login(username string, password string) error
	GetAPIVersion(ctx context.Context) (string, error)
	GetTorrents(ctx context.Context) ([]*Torrent, error)
	GetTorrent(ctx context.Context, hash string) (*Torrent, error)
	AddTorrentByURL(ctx context.Context, url string, torrent *torrentv1alpha1.Torrent) error
	StopTorrent(ctx context.Context, hash string) error
	ResumeTorrent(ctx context.Context, hash string) error
	DeleteTorrent(ctx context.Context, hash string, keepFiles bool) error
	RenameTorrent(ctx context.Context, hash string, name string) error
	MoveTorrent(ctx context.Context, hash string, destination string) error
	GetTorrentStatus(ctx context.Context, hash string) (torrentv1alpha1.TorrentStatus, error)
}
