package daemon

import (
	"encoding/json"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/loupax/secret-sauce/internal/ipc"
	"github.com/loupax/secret-sauce/internal/service"
)

// Server holds daemon state.
type Server struct {
	socketPath string
	timeout    time.Duration
	svc        *service.LocalVaultService
	listener   net.Listener
	idleTimer  *time.Timer
	mu         sync.Mutex
}

func NewServer(socketPath string, timeout time.Duration) *Server {
	return &Server{
		socketPath: socketPath,
		timeout:    timeout,
		svc:        service.NewLocalVaultService(),
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
		secrets, err := s.svc.ReadAllSecrets(req.VaultDir)
		if err != nil {
			resp = ipc.Response{OK: false, Error: err.Error()}
		} else {
			resp = ipc.Response{OK: true, Secrets: secrets}
		}

	case ipc.OpWrite:
		if err := s.svc.WriteSecret(req.VaultDir, req.Key, req.Value); err != nil {
			resp = ipc.Response{OK: false, Error: err.Error()}
		} else {
			resp = ipc.Response{OK: true}
		}

	case ipc.OpDelete:
		if err := s.svc.DeleteSecret(req.VaultDir, req.Key); err != nil {
			resp = ipc.Response{OK: false, Error: err.Error()}
		} else {
			resp = ipc.Response{OK: true}
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
