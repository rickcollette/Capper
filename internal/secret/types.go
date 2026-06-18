package secret

// Secret is an encrypted key-value stored locally.
type Secret struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Project     string `json:"project"`
	Description string `json:"description,omitempty"`
	Ciphertext  []byte `json:"ciphertext"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}
