package helper

import (
	"context"
	"fmt"
	torrentv1alpha1 "github.com/alphayax/torrent-operator/api/v1alpha1"
	"github.com/hekmon/transmissionrpc/v3"
	"net/url"
	"strconv"
	"time"
)

type TransmissionServer struct {
	transmissionRpcUrl string // Something like "127.0.0.1:9091" // TODO: Do better. Handle url + scheme
	client             *transmissionrpc.Client
}

func NewTransmissionServer(btServerSpec torrentv1alpha1.BtServerSpec) *TransmissionServer {
	return &TransmissionServer{
		transmissionRpcUrl: btServerSpec.ServerUri,
	}
}

func (s *TransmissionServer) Login(username string, password string) error {
	transmissionUrl := fmt.Sprintf("%s/transmission/rpc", s.transmissionRpcUrl)
	endpoint, err := url.Parse(transmissionUrl)
	endpoint.User = url.UserPassword(username, password)

	if err != nil {
		return err
	}

	s.client, err = transmissionrpc.New(endpoint, nil)
	if err != nil {
		panic(err)
	}
	return err
}

func (s *TransmissionServer) GetAPIVersion() (string, error) {
	// TODO: Context
	ctx := context.TODO()
	ok, serverVersion, serverMinimumVersion, err := s.client.RPCVersion(ctx)
	if err != nil {
		return "", err
	}
	version := strconv.FormatInt(serverVersion, 10)
	if !ok {
		err = fmt.Errorf(
			"remote transmission RPC version (v%d) is incompatible with the transmission library (v%d): remote needs at least v%d",
			serverVersion, transmissionrpc.RPCVersion, serverMinimumVersion,
		)
		return version, err
	}
	return version, nil
}

func (s *TransmissionServer) GetTorrents() ([]*Torrent, error) {
	// TODO: Context
	trTorrents, err := s.client.TorrentGetAll(context.TODO())
	if err != nil {
		return nil, err
	}

	torrents := make([]*Torrent, len(trTorrents))
	for i, t := range trTorrents {
		torrents[i] = &Torrent{
			Hash:        *t.HashString,
			Name:        *t.Name,
			DownloadDir: *t.DownloadDir,
			IsPaused:    *t.Status == transmissionrpc.TorrentStatusStopped,
			Tags:        *t.Comment,
		}
	}
	return torrents, nil
}

func (s *TransmissionServer) getTransmissionTorrentByHash(ctx context.Context, hash string) (*transmissionrpc.Torrent, error) {
	trTorrents, err := s.client.TorrentGetAllForHashes(ctx, []string{hash})
	if err != nil {
		return nil, err
	}
	if len(trTorrents) == 0 {
		return nil, fmt.Errorf("torrent with hash '%s' not found", hash)
	}
	return &trTorrents[0], nil
}

func (s *TransmissionServer) GetTorrent(hash string) (*Torrent, error) {
	// TODO: Context
	trTorrent, err := s.getTransmissionTorrentByHash(context.TODO(), hash)
	if err != nil {
		return nil, err
	}
	return &Torrent{
		Hash:        *trTorrent.HashString,
		Name:        *trTorrent.Name,
		DownloadDir: *trTorrent.DownloadDir,
		IsPaused:    *trTorrent.Status == transmissionrpc.TorrentStatusStopped,
		Tags:        *trTorrent.Comment,
	}, nil
}

func (s *TransmissionServer) AddTorrentByURL(torrentUrl string, torrent *torrentv1alpha1.Torrent) error {
	// TODO: Context
	_, err := s.client.TorrentAdd(context.TODO(), transmissionrpc.TorrentAddPayload{
		DownloadDir: &torrent.Spec.DownloadDir,
		Paused:      &torrent.Spec.Paused,
		Filename:    &torrentUrl,
	})
	return err
}

func (s *TransmissionServer) StopTorrent(hash string) error {
	return s.client.TorrentStopHashes(context.TODO(), []string{hash})
}

func (s *TransmissionServer) ResumeTorrent(hash string) error {
	return s.client.TorrentStartHashes(context.TODO(), []string{hash})
}

func (s *TransmissionServer) DeleteTorrent(hash string, keepFiles bool) error {
	trTorrent, err := s.getTransmissionTorrentByHash(context.TODO(), hash)
	if err != nil {
		return err
	}
	return s.client.TorrentRemove(context.TODO(), transmissionrpc.TorrentRemovePayload{
		DeleteLocalData: !keepFiles,
		IDs:             []int64{*trTorrent.ID},
	})
}

func (s *TransmissionServer) RenameTorrent(hash string, name string) error {
	return s.client.TorrentRenamePathHash(context.TODO(), hash, "", name)
}

func (s *TransmissionServer) MoveTorrent(hash string, destination string) error {
	return s.client.TorrentRenamePathHash(context.TODO(), hash, destination, "")
}

func (s *TransmissionServer) GetTorrentStatus(hash string) (torrentv1alpha1.TorrentStatus, error) {
	// TODO: Context
	trTorrent, err := s.getTransmissionTorrentByHash(context.TODO(), hash)
	if err != nil {
		return torrentv1alpha1.TorrentStatus{}, err
	}

	var seederCount, leecherCount int64
	for _, trackerStats := range trTorrent.TrackerStats {
		seederCount += trackerStats.SeederCount
		leecherCount += trackerStats.LeecherCount
	}

	return torrentv1alpha1.TorrentStatus{
		AddedOn:  trTorrent.AddedDate.String(),
		State:    getHumanReadableStatus(*trTorrent.Status),
		Ratio:    strconv.FormatFloat(*trTorrent.UploadRatio, 'f', -1, 64),
		Progress: strconv.FormatFloat(*trTorrent.PercentDone, 'f', -1, 64),
		Size:     (*trTorrent.TotalSize).GetHumanSizeRepresentation(),
		Eta:      (time.Duration(*trTorrent.ETA) * time.Second).String(),
		Speed: torrentv1alpha1.TorrentStatusSpeed{
			DlSpeed: int(*trTorrent.RateDownload),
			UpSpeed: int(*trTorrent.RateUpload),
		},
		Peers: torrentv1alpha1.TorrentStatusPeers{
			Leechers: fmt.Sprintf("%d/%d", *trTorrent.PeersGettingFromUs, leecherCount),
			Seeders:  fmt.Sprintf("%d/%d", *trTorrent.PeersSendingToUs, seederCount),
		},
		Data: torrentv1alpha1.TorrentStatusData{
			Downloaded: HumanReadableSize(*trTorrent.DownloadedEver),
			Uploaded:   HumanReadableSize(*trTorrent.UploadedEver),
		},
	}, nil
}

func getHumanReadableStatus(status transmissionrpc.TorrentStatus) string {
	switch status {
	case transmissionrpc.TorrentStatusStopped:
		return "Stopped"
	case transmissionrpc.TorrentStatusCheckWait:
		return "CheckWait"
	case transmissionrpc.TorrentStatusCheck:
		return "Checking"
	case transmissionrpc.TorrentStatusDownloadWait:
		return "DownloadWait"
	case transmissionrpc.TorrentStatusDownload:
		return "Downloading"
	case transmissionrpc.TorrentStatusSeedWait:
		return "SeedWait"
	case transmissionrpc.TorrentStatusSeed:
		return "Seeding"
	case transmissionrpc.TorrentStatusIsolated:
		return "Isolated"
	default:
		return "Unknown"
	}
}
