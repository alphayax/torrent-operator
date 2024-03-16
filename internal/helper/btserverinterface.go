package helper

import (
	torrentv1alpha1 "github.com/alphayax/torrent-operator/api/v1alpha1"
)

type BtServerInterface interface {
	Login(username string, password string) error
	GetAPIVersion() (string, error)
	GetTorrents() ([]*Torrent, error)
	GetTorrent(hash string) (*Torrent, error)
	AddTorrentByURL(url string, torrent *torrentv1alpha1.Torrent) error
	StopTorrent(hash string) error
	ResumeTorrent(hash string) error
	DeleteTorrent(hash string, keepFiles bool) error
	RenameTorrent(hash string, name string) error
	MoveTorrent(hash string, destination string) error
	GetTorrentStatus(hash string) (torrentv1alpha1.TorrentStatus, error)
}
