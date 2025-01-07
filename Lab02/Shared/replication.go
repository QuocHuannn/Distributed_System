package shared

type ServerInfo struct {
	ID        int
	Address   string
	IsPrimary bool
}

type ElectionMessage struct {
	Type     string // "Election", "Victory", "OK"
	ServerID int
}

type ReplicationMessage struct {
	Type      string // "Sync", "SyncAck"
	Key       string
	Value     string
	Timestamp int64
}

const (
	Election = "Election"
	Victory  = "Victory"
	Sync     = "Sync" 
	SyncAck  = "SyncAck"
)
