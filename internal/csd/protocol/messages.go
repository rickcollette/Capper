package csdprotocol

// Message type bytes — prefix every CSDP stream frame.
const (
	MsgHello        = byte(0x01)
	MsgHelloOK      = byte(0x02)
	MsgError        = byte(0xFF)

	MsgMount        = byte(0x03)
	MsgMountOK      = byte(0x04)

	MsgLookup       = byte(0x10)
	MsgLookupResp   = byte(0x11)
	MsgGetattr      = byte(0x12)
	MsgGetattrResp  = byte(0x13)
	MsgReaddir      = byte(0x14)
	MsgReaddirResp  = byte(0x15)

	MsgRead         = byte(0x20)
	MsgReadResp     = byte(0x21)
	MsgWrite        = byte(0x22)
	MsgWriteResp    = byte(0x23)

	MsgCreate       = byte(0x30)
	MsgCreateResp   = byte(0x31)
	MsgMkdir        = byte(0x32)
	MsgMkdirResp    = byte(0x33)
	MsgUnlink       = byte(0x34)
	MsgUnlinkResp   = byte(0x35)
	MsgRename       = byte(0x36)
	MsgRenameResp   = byte(0x37)
	MsgTruncate     = byte(0x38)
	MsgTruncateResp = byte(0x39)
	MsgFsync        = byte(0x3A)
	MsgFsyncResp    = byte(0x3B)

	MsgLeaseAcquire = byte(0x40)
	MsgLeaseAcqResp = byte(0x41)
	MsgLeaseRenew   = byte(0x42)
	MsgLeaseRenResp = byte(0x43)
	MsgLeaseRelease = byte(0x44)
	MsgLeaseRelResp = byte(0x45)

	MsgPing         = byte(0x50)
	MsgPong         = byte(0x51)
)

// Error codes returned in ErrorMsg.Code.
const (
	ErrCodeNotFound      = "ERR_NOT_FOUND"
	ErrCodeStaleEpoch    = "ERR_STALE_EPOCH"
	ErrCodeLeaseConflict = "ERR_LEASE_CONFLICT"
	ErrCodeAccessDenied  = "ERR_ACCESS_DENIED"
	ErrCodeInternal      = "ERR_INTERNAL"
	ErrCodeReadonly      = "ERR_READONLY"
)

// CSDP protocol version.
const ProtocolVersion = 1

// ---- handshake messages -----------------------------------------------------

type HelloMsg struct {
	Protocol   string   `json:"p"`
	Version    int      `json:"v"`
	ClientID   string   `json:"cid"`
	NodeID     string   `json:"nid"`
	InstanceID string   `json:"iid"`
	VolumeID   string   `json:"vid"`
	Caps       []string `json:"caps,omitempty"`
}

type HelloOKMsg struct {
	SessionID     string   `json:"sid"`
	Epoch         int64    `json:"epoch"`
	VolumeMode    string   `json:"mode"`
	MaxReadBytes  int      `json:"maxr"`
	MaxWriteBytes int      `json:"maxw"`
	LeaseTTLSecs  int      `json:"lttl"`
	Features      []string `json:"feat,omitempty"`
}

type ErrorMsg struct {
	Code    string `json:"code"`
	Message string `json:"msg"`
	Epoch   int64  `json:"epoch,omitempty"`
}

// ---- metadata messages ------------------------------------------------------

type LookupMsg struct {
	SessionID string `json:"sid"`
	ParentID  string `json:"pid"`
	Name      string `json:"name"`
}

type InodeMsg struct {
	ID         string `json:"id"`
	ParentID   string `json:"pid"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Size       int64  `json:"size"`
	Mode       uint32 `json:"mode"`
	UID        uint32 `json:"uid"`
	GID        uint32 `json:"gid"`
	ModifiedAt string `json:"mtime"`
	CreatedAt  string `json:"ctime"`
}

type GetattrMsg struct {
	SessionID string `json:"sid"`
	InodeID   string `json:"ino"`
}

type ReaddirMsg struct {
	SessionID string `json:"sid"`
	DirID     string `json:"dir"`
}

type ReaddirRespMsg struct {
	Entries []InodeMsg `json:"entries"`
}

type CreateMsg struct {
	SessionID string `json:"sid"`
	ParentID  string `json:"pid"`
	Name      string `json:"name"`
	Mode      uint32 `json:"mode"`
}

type MkdirMsg struct {
	SessionID string `json:"sid"`
	ParentID  string `json:"pid"`
	Name      string `json:"name"`
	Mode      uint32 `json:"mode"`
}

type UnlinkMsg struct {
	SessionID string `json:"sid"`
	ParentID  string `json:"pid"`
	InodeID   string `json:"ino"`
	Name      string `json:"name"`
}

type RenameMsg struct {
	SessionID   string `json:"sid"`
	InodeID     string `json:"ino"`
	OldParentID string `json:"opid"`
	OldName     string `json:"oname"`
	NewParentID string `json:"npid"`
	NewName     string `json:"nname"`
}

type TruncateMsg struct {
	SessionID string `json:"sid"`
	InodeID   string `json:"ino"`
	NewSize   int64  `json:"size"`
}

type FsyncMsg struct {
	SessionID string `json:"sid"`
	InodeID   string `json:"ino"`
}

// ---- data messages ----------------------------------------------------------

type ReadMsg struct {
	SessionID string `json:"sid"`
	InodeID   string `json:"ino"`
	Offset    int64  `json:"off"`
	Length    int    `json:"len"`
}

type ReadRespMsg struct {
	Data   []byte `json:"data"`
	EOF    bool   `json:"eof"`
}

type WriteMsg struct {
	SessionID string `json:"sid"`
	ClientID  string `json:"cid"`
	Epoch     int64  `json:"epoch"`
	InodeID   string `json:"ino"`
	Offset    int64  `json:"off"`
	OpSeq     int64  `json:"seq"`
	Data      []byte `json:"data"`
}

type WriteRespMsg struct {
	Written int `json:"n"`
}

// ---- lease messages ---------------------------------------------------------

type LeaseAcquireMsg struct {
	SessionID  string `json:"sid"`
	InodeID    string `json:"ino"`
	LeaseType  string `json:"type"`
	RangeStart int64  `json:"rs"`
	RangeEnd   int64  `json:"re"`
	Epoch      int64  `json:"epoch"`
}

type LeaseAcqRespMsg struct {
	LeaseID   string `json:"lid"`
	ExpiresAt string `json:"exp"`
	Epoch     int64  `json:"epoch"`
}

type LeaseRenewMsg struct {
	SessionID string `json:"sid"`
	LeaseID   string `json:"lid"`
}

type LeaseReleaseMsg struct {
	SessionID string `json:"sid"`
	LeaseID   string `json:"lid"`
}

// ---- ping/pong --------------------------------------------------------------

type PingMsg struct {
	Seq int64 `json:"seq"`
}

type PongMsg struct {
	Seq   int64 `json:"seq"`
	Epoch int64 `json:"epoch"`
}
