package vault

import "testing"

func TestValidSecretTypeMap(t *testing.T) {
	if !ValidSecretType(SecretTypeMap) {
		t.Error("expected SecretTypeMap to be valid")
	}
}
