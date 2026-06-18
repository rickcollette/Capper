package types

type ImageRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Path      string `json:"path"`
	CreatedAt string `json:"createdAt"`
	SizeBytes int64  `json:"sizeBytes"`
	Digest    string `json:"digest"`
	Missing   bool   `json:"missing,omitempty"`
}
