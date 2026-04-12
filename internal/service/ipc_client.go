package service

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/loupax/secret-sauce/internal/ipc"
	"github.com/loupax/secret-sauce/internal/vault"
)

// IPCVaultService satisfies VaultService by sending JSON requests over the Unix socket.
type IPCVaultService struct {
	socketPath string
}

func NewIPCVaultService(socketPath string) *IPCVaultService {
	return &IPCVaultService{socketPath: socketPath}
}

func (s *IPCVaultService) dial() (net.Conn, error) {
	conn, err := net.DialTimeout("unix", s.socketPath, 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("connect to daemon: %w", err)
	}
	return conn, nil
}

func (s *IPCVaultService) roundTrip(req ipc.Request) (ipc.Response, error) {
	conn, err := s.dial()
	if err != nil {
		return ipc.Response{}, err
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return ipc.Response{}, fmt.Errorf("send request: %w", err)
	}

	var resp ipc.Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return ipc.Response{}, fmt.Errorf("decode response: %w", err)
	}
	return resp, nil
}

func (s *IPCVaultService) ListSecretNames(vaultDir string) ([]string, error) {
	resp, err := s.roundTrip(ipc.Request{Op: ipc.OpListNames, VaultDir: vaultDir})
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("daemon error: %s", resp.Error)
	}
	return resp.Names, nil
}

func (s *IPCVaultService) ReadAllSecrets(vaultDir string) (map[string]vault.SecretInfo, error) {
	resp, err := s.roundTrip(ipc.Request{Op: ipc.OpReadAll, VaultDir: vaultDir})
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("daemon error: %s", resp.Error)
	}
	result := make(map[string]vault.SecretInfo, len(resp.Secrets))
	for k, meta := range resp.Secrets {
		result[k] = vault.SecretInfo{Data: meta.Data}
	}
	return result, nil
}

func (s *IPCVaultService) ReadSecret(vaultDir, key string) (vault.SecretInfo, error) {
	resp, err := s.roundTrip(ipc.Request{Op: ipc.OpReadOne, VaultDir: vaultDir, Key: key})
	if err != nil {
		return vault.SecretInfo{}, err
	}
	if !resp.OK {
		return vault.SecretInfo{}, fmt.Errorf("daemon error: %s", resp.Error)
	}
	if resp.Secret == nil {
		return vault.SecretInfo{}, vault.ErrKeyNotFound
	}
	return vault.SecretInfo{Data: resp.Secret.Data}, nil
}

func (s *IPCVaultService) WriteSecret(vaultDir, key string, data map[string]string) error {
	resp, err := s.roundTrip(ipc.Request{
		Op:       ipc.OpWrite,
		VaultDir: vaultDir,
		Key:      key,
		Data:     data,
	})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("daemon error: %s", resp.Error)
	}
	return nil
}

func (s *IPCVaultService) DeleteSecret(vaultDir, key string) error {
	resp, err := s.roundTrip(ipc.Request{Op: ipc.OpDelete, VaultDir: vaultDir, Key: key})
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf("daemon error: %s", resp.Error)
	}
	return nil
}

func (s *IPCVaultService) GetPublicKey(vaultDir string) (string, error) {
	resp, err := s.roundTrip(ipc.Request{Op: ipc.OpGetPubKey, VaultDir: vaultDir})
	if err != nil {
		return "", err
	}
	if !resp.OK {
		return "", fmt.Errorf("daemon error: %s", resp.Error)
	}
	return resp.PubKey, nil
}
