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
	transmissionRpcUrl string
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

func (s *TransmissionServer) GetAPIVersion(ctx context.Context) (string, error) {
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

func (s *TransmissionServer) GetTorrents(ctx context.Context) ([]*Torrent, error) {
	trTorrents, err := s.client.TorrentGetAll(ctx)
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

func (s *TransmissionServer) GetTorrent(ctx context.Context, hash string) (*Torrent, error) {
	trTorrent, err := s.getTransmissionTorrentByHash(ctx, hash)
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

func (s *TransmissionServer) AddTorrentByURL(ctx context.Context, torrentUrl string, torrent *torrentv1alpha1.Torrent) error {
	_, err := s.client.TorrentAdd(ctx, transmissionrpc.TorrentAddPayload{
		DownloadDir: &torrent.Spec.DownloadDir,
		Paused:      &torrent.Spec.Paused,
		Filename:    &torrentUrl,
	})
	return err
}

func (s *TransmissionServer) StopTorrent(ctx context.Context, hash string) error {
	return s.client.TorrentStopHashes(ctx, []string{hash})
}

func (s *TransmissionServer) ResumeTorrent(ctx context.Context, hash string) error {
	return s.client.TorrentStartHashes(ctx, []string{hash})
}

func (s *TransmissionServer) DeleteTorrent(ctx context.Context, hash string, keepFiles bool) error {
	trTorrent, err := s.getTransmissionTorrentByHash(ctx, hash)
	if err != nil {
		return err
	}
	return s.client.TorrentRemove(ctx, transmissionrpc.TorrentRemovePayload{
		DeleteLocalData: !keepFiles,
		IDs:             []int64{*trTorrent.ID},
	})
}

func (s *TransmissionServer) RenameTorrent(ctx context.Context, hash string, name string) error {
	trTorrent, err := s.getTransmissionTorrentByHash(ctx, hash)
	if err != nil {
		return err
	}
	return s.client.TorrentRenamePath(ctx, *trTorrent.ID, *trTorrent.Name, name)
}

func (s *TransmissionServer) MoveTorrent(ctx context.Context, hash string, destination string) error {
	return s.client.TorrentSetLocationHash(ctx, hash, destination, true)
}

func (s *TransmissionServer) GetTorrentStatus(ctx context.Context, hash string) (*torrentv1alpha1.TorrentStatus, error) {
	trTorrent, err := s.getTransmissionTorrentByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	var seederCount, leecherCount int64
	for _, trackerStats := range trTorrent.TrackerStats {
		seederCount += trackerStats.SeederCount
		leecherCount += trackerStats.LeecherCount
	}

	return &torrentv1alpha1.TorrentStatus{
		AddedOn:  trTorrent.AddedDate.String(),
		State:    s.getHumanReadableStatus(*trTorrent.Status).String(),
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

func (s *TransmissionServer) getHumanReadableStatus(status transmissionrpc.TorrentStatus) TorrentState {
	switch status {
	case transmissionrpc.TorrentStatusStopped:
		return StatePaused
	case transmissionrpc.TorrentStatusCheckWait:
		return StateChecking
	case transmissionrpc.TorrentStatusCheck:
		return StateChecking
	case transmissionrpc.TorrentStatusDownloadWait:
		return StateDownloading
	case transmissionrpc.TorrentStatusDownload:
		return StateDownloading
	case transmissionrpc.TorrentStatusSeedWait:
		return StateSeeding
	case transmissionrpc.TorrentStatusSeed:
		return StateSeeding
	case transmissionrpc.TorrentStatusIsolated:
		return StateUnknown
	default:
		return StateUnknown
	}
}
