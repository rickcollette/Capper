package replication

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	quic "github.com/quic-go/quic-go"
)

// electionMsg is the wire format for election RPC messages over QUIC.
type electionMsg struct {
	Type     string `json:"type"` // "election", "coordinator", "heartbeat", "ok"
	NodeID   string `json:"nodeID"`
	LeaderID string `json:"leaderID,omitempty"`
	Term     uint64 `json:"term"`
}

// QUICElectionTransport implements Transport using QUIC streams for
// low-latency election and heartbeat messages between CSD nodes.
// Each RPC opens a new unidirectional stream so messages don't head-of-line block.
type QUICElectionTransport struct {
	tlsCfg *tls.Config
	mu     sync.Mutex
	conns  map[string]*quic.Conn // peer addr → live connection
}

// NewQUICElectionTransport creates a transport using the given TLS config.
func NewQUICElectionTransport(tlsCfg *tls.Config) *QUICElectionTransport {
	return &QUICElectionTransport{
		tlsCfg: tlsCfg,
		conns:  make(map[string]*quic.Conn),
	}
}

// SendElection sends an ELECTION message to peer and waits for an OK response.
// Returns true if the peer yielded (sent OK), false if no response or peer has lower ID.
func (t *QUICElectionTransport) SendElection(ctx context.Context, peer, nodeID string, term uint64) (bool, error) {
	conn, err := t.dial(ctx, peer)
	if err != nil {
		return false, err
	}
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return false, fmt.Errorf("election: open stream to %s: %w", peer, err)
	}
	defer stream.Close()

	msg := electionMsg{Type: "election", NodeID: nodeID, Term: term}
	if err := writeElectionMsg(stream, msg); err != nil {
		return false, err
	}

	// Wait for OK with a short deadline.
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(200 * time.Millisecond)
	}
	_ = stream.SetReadDeadline(deadline)

	var reply electionMsg
	if err := readElectionMsg(stream, &reply); err != nil {
		return false, nil // no response — peer may be down
	}
	return reply.Type == "ok", nil
}

// SendCoordinator broadcasts a COORDINATOR message to peer.
func (t *QUICElectionTransport) SendCoordinator(ctx context.Context, peer, leaderID string, term uint64) error {
	conn, err := t.dial(ctx, peer)
	if err != nil {
		return err
	}
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("coordinator: open stream to %s: %w", peer, err)
	}
	defer stream.Close()
	return writeElectionMsg(stream, electionMsg{Type: "coordinator", LeaderID: leaderID, Term: term})
}

// SendHeartbeat sends a heartbeat to peer for the given term.
func (t *QUICElectionTransport) SendHeartbeat(ctx context.Context, peer string, term uint64) error {
	conn, err := t.dial(ctx, peer)
	if err != nil {
		return err
	}
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("heartbeat: open stream to %s: %w", peer, err)
	}
	defer stream.Close()
	return writeElectionMsg(stream, electionMsg{Type: "heartbeat", Term: term})
}

// dial returns a cached or new QUIC connection to addr.
func (t *QUICElectionTransport) dial(ctx context.Context, addr string) (*quic.Conn, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if conn, ok := t.conns[addr]; ok {
		select {
		case <-conn.Context().Done():
			delete(t.conns, addr)
		default:
			return conn, nil
		}
	}
	conn, err := quic.DialAddr(ctx, addr, t.tlsCfg, &quic.Config{
		MaxIdleTimeout:  30 * time.Second,
		KeepAlivePeriod: 10 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("election transport: dial %s: %w", addr, err)
	}
	t.conns[addr] = conn
	return conn, nil
}

func writeElectionMsg(w interface{ Write([]byte) (int, error) }, msg electionMsg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	// 4-byte little-endian length prefix.
	l := uint32(len(data))
	buf.WriteByte(byte(l))
	buf.WriteByte(byte(l >> 8))
	buf.WriteByte(byte(l >> 16))
	buf.WriteByte(byte(l >> 24))
	buf.Write(data)
	_, err = w.Write(buf.Bytes())
	return err
}

func readElectionMsg(r interface{ Read([]byte) (int, error) }, out *electionMsg) error {
	var lenBuf [4]byte
	if _, err := readFull(r, lenBuf[:]); err != nil {
		return err
	}
	n := uint32(lenBuf[0]) | uint32(lenBuf[1])<<8 | uint32(lenBuf[2])<<16 | uint32(lenBuf[3])<<24
	if n > 1<<16 {
		return fmt.Errorf("election message too large: %d", n)
	}
	data := make([]byte, n)
	if _, err := readFull(r, data); err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func readFull(r interface{ Read([]byte) (int, error) }, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := r.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
