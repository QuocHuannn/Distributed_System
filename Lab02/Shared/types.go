package shared

const (
	OK         = "OK"
	ErrNoKey   = "ErrNoKey"
	NotPrimary = "NotPrimary"
)

type Err string

type PutArgs struct {
	Key   string
	Value string
}

type PutReply struct {
	Err Err
}

type GetArgs struct {
	Key string
}

type GetReply struct {
	Err   Err
	Value string
}
