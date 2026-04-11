package cmd

import (
	"slices"
	"testing"

	"github.com/loupax/secret-sauce/internal/vault"
)

// buildCombinedEnvFromManifestEnv replicates env-injection logic for unit testing.
// envMap: envVar -> secretName, secrets: name -> SecretInfo
func buildCombinedEnvFromManifestEnv(envMap map[string]string, secrets map[string]vault.SecretInfo) []string {
	var combined []string
	for envVar, secretName := range envMap {
		info, ok := secrets[secretName]
		if !ok {
			continue
		}
		combined = append(combined, envVar+"="+info.Data["value"])
	}
	return combined
}

func TestRunInjectsEnvSecrets(t *testing.T) {
	secrets := map[string]vault.SecretInfo{
		"db-secret": {Data: map[string]string{"value": "postgres://localhost"}},
	}
	envMap := map[string]string{
		"DB_URL": "db-secret",
	}

	combined := buildCombinedEnvFromManifestEnv(envMap, secrets)

	if !slices.Contains(combined, "DB_URL=postgres://localhost") {
		t.Error("expected DB_URL to be injected into env")
	}
}

func TestRunDoesNotInjectUnmappedSecrets(t *testing.T) {
	secrets := map[string]vault.SecretInfo{
		"cfg-secret": {Data: map[string]string{"host": "localhost", "port": "5432"}},
	}
	// cfg-secret is not wired in the manifest env map
	envMap := map[string]string{}

	combined := buildCombinedEnvFromManifestEnv(envMap, secrets)

	for _, entry := range combined {
		_ = entry
		// nothing should appear
	}
	if len(combined) != 0 {
		t.Errorf("expected empty combined env, got: %v", combined)
	}
}
