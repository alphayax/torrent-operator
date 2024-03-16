package helper

import (
	"fmt"
	qbt "github.com/KnutZuidema/go-qbittorrent"
	"github.com/KnutZuidema/go-qbittorrent/pkg/model"
	torrentv1alpha1 "github.com/alphayax/torrent-operator/api/v1alpha1"
	"github.com/sirupsen/logrus"
	"strconv"
	"time"
)

type QBittorrentServer struct {
	client *qbt.Client
}

func NewQBittorrentServer(btServerSpec torrentv1alpha1.BtServerSpec) *QBittorrentServer {
	return &QBittorrentServer{
		client: qbt.NewClient(btServerSpec.ServerUri, logrus.New()),
	}
}

func (q *QBittorrentServer) Login(username string, password string) error {
	return q.client.Login(username, password)
}

func (q *QBittorrentServer) GetAPIVersion() (string, error) {
	return q.client.Application.GetAPIVersion()
}

func (q *QBittorrentServer) GetTorrents() ([]*Torrent, error) {
	qbtTorrents, err := q.client.Torrent.GetList(nil)
	if err != nil {
		return nil, err
	}

	torrents := make([]*Torrent, len(qbtTorrents))
	for i, qbtTorrent := range qbtTorrents {
		torrents[i] = &Torrent{
			Hash:        qbtTorrent.Hash,
			Tags:        qbtTorrent.Tags,
			Name:        qbtTorrent.Name,
			DownloadDir: qbtTorrent.SavePath,
			IsPaused:    qbtTorrent.State == "pausedUP" || qbtTorrent.State == "pausedDL",
		}
	}

	return torrents, nil
}

func (q *QBittorrentServer) GetTorrent(hash string) (*Torrent, error) {
	list, err := q.client.Torrent.GetList(&model.GetTorrentListOptions{Hashes: hash})
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("torrent with hash '%s' not found", hash)
	}
	qbtTorrent := list[0]
	return &Torrent{
		Hash:        qbtTorrent.Hash,
		Tags:        qbtTorrent.Tags,
		Name:        qbtTorrent.Name,
		DownloadDir: qbtTorrent.SavePath,
		IsPaused:    qbtTorrent.State == "pausedUP" || qbtTorrent.State == "pausedDL",
	}, nil
}

func (q *QBittorrentServer) AddTorrentByURL(torrentUrl string, torrent *torrentv1alpha1.Torrent) error {
	URLs := []string{torrentUrl}
	options := &model.AddTorrentsOptions{
		Tags:     fmt.Sprintf("k8s-%s", torrent.Name),
		Savepath: torrent.Spec.DownloadDir,
		Rename:   torrent.Spec.Name,
		Paused:   strconv.FormatBool(torrent.Spec.Paused),
	}
	return q.client.Torrent.AddURLs(URLs, options)
}

func (q *QBittorrentServer) StopTorrent(hash string) error {
	return q.client.Torrent.StopTorrents([]string{hash})
}

func (q *QBittorrentServer) ResumeTorrent(hash string) error {
	return q.client.Torrent.ResumeTorrents([]string{hash})
}

func (q *QBittorrentServer) DeleteTorrent(hash string, keepFiles bool) error {
	return q.client.Torrent.DeleteTorrents([]string{hash}, !keepFiles)
}

func (q *QBittorrentServer) RenameTorrent(hash string, name string) error {
	return q.client.Torrent.SetName(hash, name)
}

func (q *QBittorrentServer) MoveTorrent(hash string, destination string) error {
	return q.client.Torrent.SetLocations([]string{hash}, destination)
}

func (q *QBittorrentServer) GetTorrentStatus(hash string) (torrentv1alpha1.TorrentStatus, error) {
	list, err := q.client.Torrent.GetList(&model.GetTorrentListOptions{Hashes: hash})
	if err != nil {
		return torrentv1alpha1.TorrentStatus{}, err
	}
	if len(list) == 0 {
		return torrentv1alpha1.TorrentStatus{}, fmt.Errorf("torrent with hash '%s' not found", hash)
	}
	if err != nil {
		return torrentv1alpha1.TorrentStatus{}, err
	}
	qbTorrent := list[0]
	return torrentv1alpha1.TorrentStatus{
		AddedOn:  time.Unix(int64(qbTorrent.AddedOn), 0).Format(time.RFC3339),
		State:    string(qbTorrent.State),
		Progress: fmt.Sprintf("%.2f%%", qbTorrent.Progress*100),
		Ratio:    fmt.Sprintf("%.3f", qbTorrent.Ratio),
		Speed: torrentv1alpha1.TorrentStatusSpeed{
			DlSpeed: qbTorrent.Dlspeed,
			UpSpeed: qbTorrent.Upspeed,
		},
		Eta:  (time.Duration(qbTorrent.Eta) * time.Second).String(),
		Size: HumanReadableSize(int64(qbTorrent.Size)),
		Data: torrentv1alpha1.TorrentStatusData{
			Downloaded: HumanReadableSize(qbTorrent.Downloaded),
			Uploaded:   HumanReadableSize(qbTorrent.Uploaded),
		},
		Peers: torrentv1alpha1.TorrentStatusPeers{
			Leechers: fmt.Sprintf("%d/%d", qbTorrent.NumLeechs, qbTorrent.NumIncomplete),
			Seeders:  fmt.Sprintf("%d/%d", qbTorrent.NumSeeds, qbTorrent.NumComplete),
		},
	}, nil
}
