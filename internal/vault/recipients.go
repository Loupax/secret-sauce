package vault

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
)

func ReadRecipients(vaultDir string) ([]age.Recipient, error) {
	path := filepath.Join(vaultDir, "vault_recipients.txt")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open recipients file: %w", err)
	}
	defer f.Close()

	var recipients []age.Recipient
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		r, err := age.ParseX25519Recipient(trimmed)
		if err != nil {
			return nil, fmt.Errorf("invalid recipient %q: %w", line, err)
		}
		recipients = append(recipients, r)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading recipients file: %w", err)
	}
	return recipients, nil
}

func AppendRecipient(vaultDir string, pubkey string) error {
	path := filepath.Join(vaultDir, "vault_recipients.txt")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return fmt.Errorf("open recipients file for append: %w", err)
	}
	if _, err := fmt.Fprintln(f, pubkey); err != nil {
		f.Close()
		return fmt.Errorf("write recipient: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close recipients file: %w", err)
	}
	return nil
}
