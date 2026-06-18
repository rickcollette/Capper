package manager

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
)

var adjectives = []string{"quiet", "bold", "silver", "steady", "bright", "swift", "calm", "green"}
var animals = []string{"raven", "wolf", "moth", "lynx", "hawk", "otter", "fox", "seal"}

func randomHexID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func randomName(image string, exists func(string) (bool, error)) (string, error) {
	base := strings.TrimSuffix(image, ".cap")
	for i := 0; i < 128; i++ {
		adj, err := choice(adjectives)
		if err != nil {
			return "", err
		}
		animal, err := choice(animals)
		if err != nil {
			return "", err
		}
		name := fmt.Sprintf("%s-%s-%s", base, adj, animal)
		found, err := exists(name)
		if err != nil {
			return "", err
		}
		if !found {
			return name, nil
		}
	}
	return "", fmt.Errorf("could not generate unique instance name")
}

func choice(values []string) (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(values))))
	if err != nil {
		return "", err
	}
	return values[n.Int64()], nil
}
