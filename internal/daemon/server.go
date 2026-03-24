package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"filippo.io/age"
	"golang.org/x/sync/errgroup"

	"github.com/loupax/secret-sauce/internal/ipc"
	"github.com/loupax/secret-sauce/internal/keyring"
	"github.com/loupax/secret-sauce/internal/service"
	"github.com/loupax/secret-sauce/internal/vault"
)

// Server holds daemon state.
type Server struct {
	socketPath string
	timeout    time.Duration
	svc        *service.LocalVaultService
	listener   net.Listener
	idleTimer  *time.Timer
	mu         sync.Mutex
	index      *VaultIndex
}

func NewServer(socketPath string, timeout time.Duration) *Server {
	return &Server{
		socketPath: socketPath,
		timeout:    timeout,
		svc:        service.NewLocalVaultService(),
		index:      newVaultIndex(),
	}
}

func (s *Server) Run() error {
	// Remove stale socket if present.
	os.Remove(s.socketPath)

	ln, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return err
	}
	s.listener = ln

	// Restrict socket to owner only — security requirement.
	if err := os.Chmod(s.socketPath, 0600); err != nil {
		ln.Close()
		os.Remove(s.socketPath)
		return err
	}

	defer func() {
		ln.Close()
		os.Remove(s.socketPath)
	}()

	s.resetIdleTimer()

	for {
		conn, err := ln.Accept()
		if err != nil {
			// Listener was closed — normal shutdown path.
			return nil
		}
		s.resetIdleTimer()
		go s.handleConn(conn)
	}
}

// refreshIndexIfStale checks the vault directory's mtime and rebuilds the
// in-memory index if it has changed since the last build.
func (s *Server) refreshIndexIfStale(vaultDir string) error {
	info, err := os.Stat(vaultDir)
	if err != nil {
		return err
	}
	currentMtime := info.ModTime()

	s.index.mu.RLock()
	upToDate := !s.index.dirModTime.IsZero() && !currentMtime.After(s.index.dirModTime)
	s.index.mu.RUnlock()
	if upToDate {
		return nil
	}

	// Need to rebuild index — acquire write lock.
	s.index.mu.Lock()
	defer s.index.mu.Unlock()

	// Double-check after acquiring write lock.
	if !s.index.dirModTime.IsZero() && !currentMtime.After(s.index.dirModTime) {
		return nil
	}

	// Load identity from keyring.
	keyStr, err := keyring.Load(vaultDir)
	if err != nil {
		return err
	}

	parsedIdentity, err := age.ParseX25519Identity(keyStr)
	if err != nil {
		return fmt.Errorf("parse identity: %w", err)
	}

	// Glob all .age files.
	pattern := filepath.Join(vaultDir, "*.age")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	newEntries := make(map[string]IndexEntry, len(files))
	var mu sync.Mutex
	g, _ := errgroup.WithContext(context.Background())
	for _, f := range files {
		g.Go(func() error {
			env, err := vault.DecryptEnvelope(f, parsedIdentity)
			if err != nil {
				return nil // skip corrupt/unreadable files
			}
			uuidName := strings.TrimSuffix(filepath.Base(f), ".age")
			mu.Lock()
			newEntries[env.Name] = IndexEntry{UUID: uuidName, Envelope: env}
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	s.index.entries = newEntries
	s.index.dirModTime = currentMtime
	return nil
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	var req ipc.Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		json.NewEncoder(conn).Encode(ipc.Response{OK: false, Error: "decode request: " + err.Error()})
		return
	}

	// Reset idle timer on every request.
	s.resetIdleTimer()

	var resp ipc.Response

	switch req.Op {
	case ipc.OpPing:
		resp = ipc.Response{OK: true}

	case ipc.OpReadAll:
		if err := s.refreshIndexIfStale(req.VaultDir); err != nil {
			resp = ipc.Response{OK: false, Error: err.Error()}
		} else {
			s.index.mu.RLock()
			secrets := make(map[string]ipc.SecretMeta, len(s.index.entries))
			for name, entry := range s.index.entries {
				secrets[name] = ipc.SecretMeta{
					Type:  string(entry.Envelope.Type),
					Value: entry.Envelope.Value,
				}
			}
			s.index.mu.RUnlock()
			resp = ipc.Response{OK: true, Secrets: secrets}
		}

	case ipc.OpReadOne:
		if err := s.refreshIndexIfStale(req.VaultDir); err != nil {
			resp = ipc.Response{OK: false, Error: err.Error()}
		} else {
			s.index.mu.RLock()
			entry, found := s.index.entries[req.Key]
			s.index.mu.RUnlock()
			if !found {
				resp = ipc.Response{OK: false, Error: vault.ErrKeyNotFound.Error()}
			} else {
				meta := ipc.SecretMeta{
					Type:  string(entry.Envelope.Type),
					Value: entry.Envelope.Value,
				}
				resp = ipc.Response{OK: true, Secret: &meta}
			}
		}

	case ipc.OpWrite:
		secretType := vault.SecretType(req.Type)
		if !vault.ValidSecretType(secretType) {
			resp = ipc.Response{OK: false, Error: "invalid secret type"}
		} else if err := s.svc.WriteSecret(req.VaultDir, req.Key, req.Value, secretType); err != nil {
			resp = ipc.Response{OK: false, Error: err.Error()}
		} else {
			// Invalidate index so next read rebuilds from disk.
			s.index.mu.Lock()
			s.index.dirModTime = time.Time{} // zero = stale, forces rebuild on next read
			s.index.mu.Unlock()
			resp = ipc.Response{OK: true}
		}

	case ipc.OpDelete:
		if err := s.refreshIndexIfStale(req.VaultDir); err != nil {
			resp = ipc.Response{OK: false, Error: err.Error()}
		} else {
			s.index.mu.RLock()
			entry, found := s.index.entries[req.Key]
			s.index.mu.RUnlock()

			if !found {
				resp = ipc.Response{OK: false, Error: vault.ErrKeyNotFound.Error()}
			} else {
				filePath := filepath.Join(req.VaultDir, entry.UUID+".age")
				if err := os.Remove(filePath); err != nil {
					resp = ipc.Response{OK: false, Error: err.Error()}
				} else {
					// Invalidate index after delete.
					s.index.mu.Lock()
					s.index.dirModTime = time.Time{}
					s.index.mu.Unlock()
					resp = ipc.Response{OK: true}
				}
			}
		}

	case ipc.OpGetPubKey:
		keyStr, err := keyring.Load(req.VaultDir)
		if err != nil {
			resp = ipc.Response{OK: false, Error: err.Error()}
		} else {
			parsedIdentity, err := age.ParseX25519Identity(keyStr)
			if err != nil {
				resp = ipc.Response{OK: false, Error: fmt.Errorf("parse identity: %w", err).Error()}
			} else {
				resp = ipc.Response{OK: true, PubKey: parsedIdentity.Recipient().String()}
			}
		}

	case ipc.OpShutdown:
		json.NewEncoder(conn).Encode(ipc.Response{OK: true})
		s.Shutdown()
		return

	default:
		resp = ipc.Response{OK: false, Error: "unknown op: " + req.Op}
	}

	json.NewEncoder(conn).Encode(resp)
}

func (s *Server) resetIdleTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.idleTimer != nil {
		s.idleTimer.Stop()
	}
	s.idleTimer = time.AfterFunc(s.timeout, func() {
		log.Println("daemon shutting down")
		s.Shutdown()
	})
}

func (s *Server) Shutdown() {
	s.mu.Lock()
	if s.idleTimer != nil {
		s.idleTimer.Stop()
	}
	s.mu.Unlock()

	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.socketPath)
}
