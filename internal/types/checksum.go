package types

type Checksums struct {
	Algorithm string            `json:"algorithm"`
	Files     map[string]string `json:"files"`
}
