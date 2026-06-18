package csdprotocol

import (
	"encoding/json"
	"fmt"
	"io"
)

// Frame layout over a QUIC stream:
//   1 byte  — message type
//   4 bytes — payload length (big-endian uint32)
//   N bytes — JSON payload

// WriteFrame encodes msg as JSON and writes a typed frame to w.
func WriteFrame(w io.Writer, msgType byte, msg any) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("csdp: marshal: %w", err)
	}
	n := uint32(len(payload))
	header := []byte{
		msgType,
		byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n),
	}
	if _, err := w.Write(header); err != nil {
		return fmt.Errorf("csdp: write header: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("csdp: write payload: %w", err)
	}
	return nil
}

// ReadFrame reads one frame from r. Returns (msgType, payload, error).
func ReadFrame(r io.Reader) (byte, []byte, error) {
	header := make([]byte, 5)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, fmt.Errorf("csdp: read header: %w", err)
	}
	msgType := header[0]
	n := uint32(header[1])<<24 | uint32(header[2])<<16 | uint32(header[3])<<8 | uint32(header[4])
	if n > 64*1024*1024 {
		return 0, nil, fmt.Errorf("csdp: frame too large (%d bytes)", n)
	}
	payload := make([]byte, n)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, fmt.Errorf("csdp: read payload: %w", err)
	}
	return msgType, payload, nil
}

// Decode unmarshals payload into dst.
func Decode(payload []byte, dst any) error {
	return json.Unmarshal(payload, dst)
}

// WriteError is a helper for sending an ErrorMsg frame.
func WriteError(w io.Writer, code, message string, epoch int64) error {
	return WriteFrame(w, MsgError, ErrorMsg{Code: code, Message: message, Epoch: epoch})
}
