package Shared

const (
	OK         = "OK"
	ErrNoKey   = "ErrNoKey"
	NotPrimary = "NotPrimary"
)

type Err string

type SetArgs struct {
	Key   string
	Value string
}

type SetReply struct {
	Err Err
}
type GetArgs struct {
	Key string
}
type GetReply struct {
	Err   Err
	Value string
}

// Bully algorithm data structure
type ElectionArgs struct {
	CandidateID int
}

type ElectionReply struct {
	Success bool
}
type CoordinatorArgs struct {
	NewLeaderID int
}

type CoordinatorReply struct{}

// SyncArgs Request sync data from primary
type SyncArgs struct {
	Key   string
	Value string
}
type SyncReply struct {
	Err Err
}

type StatusArgs struct{}
type StatusReply struct {
	ID        int
	Address   string
	IsPrimary bool
}
type HeartbeatArgs struct {
	LeaderID int
}

type HeartbeatReply struct {
	OK bool
}
