package vault

import (
	"encoding/json"
	"os"

	"filippo.io/age"
	"github.com/google/uuid"
)

// SecretType is the discriminator for envelope payloads.
type SecretType string

const (
	SecretTypeEnvVar SecretType = "env_var"
)

// SecretEnvelope is the plaintext JSON payload encrypted inside every .age file.
type SecretEnvelope struct {
	Type  SecretType `json:"type"`
	Name  string     `json:"name"`
	Value string     `json:"value"`
	Tags  []string   `json:"tags"`
}

// newUUID generates a random UUID v4 string.
func newUUID() string {
	return uuid.New().String()
}

// DecryptEnvelope opens path, age-decrypts it with identity, and returns the parsed envelope.
func DecryptEnvelope(path string, identity age.Identity) (SecretEnvelope, error) {
	f, err := os.Open(path)
	if err != nil {
		return SecretEnvelope{}, err
	}
	defer f.Close()
	r, err := age.Decrypt(f, identity)
	if err != nil {
		return SecretEnvelope{}, err
	}
	var env SecretEnvelope
	if err := json.NewDecoder(r).Decode(&env); err != nil {
		return SecretEnvelope{}, err
	}
	return env, nil
}
