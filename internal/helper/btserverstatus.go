package helper

type TorrentState string

const (
	// StateError represent a torrent in error state
	StateError TorrentState = "Error"
	// StateDownloading represent a torrent in downloading state
	StateDownloading TorrentState = "Downloading"
	// StateSeeding represent a torrent in seeding state
	StateSeeding TorrentState = "Seeding"
	// StatePaused represent a torrent in paused state
	StatePaused TorrentState = "Paused"
	// StateChecking represent a torrent in checking state
	StateChecking TorrentState = "Checking"
	// StateWarming represent a torrent in warming state
	StateWarming TorrentState = "Warming"
	// StateUnknown represent a torrent in unknown state
	StateUnknown TorrentState = "Unknown"
)

func (s TorrentState) String() string {
	return string(s)
}
