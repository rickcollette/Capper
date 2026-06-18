package types

import "bytes"

func bytesReader(data []byte) *bytes.Reader {
	return bytes.NewReader(data)
}
