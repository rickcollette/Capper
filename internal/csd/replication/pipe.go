package replication

import "io"

// newPipe returns a connected (reader, writer) pair backed by io.Pipe.
// Both halves implement io.Closer.
func newPipe() (io.ReadCloser, io.WriteCloser) {
	pr, pw := io.Pipe()
	return pr, pw
}
