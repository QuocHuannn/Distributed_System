package shared

import "time"

type StatusArgs struct{}

type StatusReply struct {
	IsPrimary bool
	ServerID  int
	Address   string
}

type HeartbeatMsg struct {
	PrimaryID int64
	Timestamp int64
}

type ServerInfo struct {
	ID        int
	Address   string
	IsPrimary bool
	StartTime time.Time
}
