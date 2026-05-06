package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	nethttp "net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	LoginSourceBrowserExtension = "browser_extension"
	defaultLoginTimeout         = 5 * time.Minute
)

// LoginServerOptions configures the local auth callback server.
type LoginServerOptions struct {
	Ports   []int
	Timeout time.Duration
}

// LoginServer receives a one-time browser extension auth callback.
type LoginServer struct {
	state     string
	port      int
	expiresAt time.Time
	server    *nethttp.Server
	listener  net.Listener
	result    chan Session
	cancelled chan struct{}
	mu        sync.Mutex
	used      bool
}

// DefaultLoginPorts returns the local port range used by auth login.
func DefaultLoginPorts() []int {
	ports := make([]int, 0, 10)
	for port := 47821; port <= 47830; port++ {
		ports = append(ports, port)
	}
	return ports
}

// StartLoginServer starts a local auth callback server on the first available port.
func StartLoginServer(ctx context.Context, opts LoginServerOptions) (*LoginServer, error) {
	if len(opts.Ports) == 0 {
		opts.Ports = DefaultLoginPorts()
	}
	if opts.Timeout <= 0 {
		opts.Timeout = defaultLoginTimeout
	}
	state, err := randomState()
	if err != nil {
		return nil, err
	}
	var listener net.Listener
	var port int
	var lastErr error
	for _, candidate := range opts.Ports {
		addr := "127.0.0.1:" + strconv.Itoa(candidate)
		listener, lastErr = net.Listen("tcp", addr)
		if lastErr == nil {
			port = candidate
			if candidate == 0 {
				if tcp, ok := listener.Addr().(*net.TCPAddr); ok {
					port = tcp.Port
				}
			}
			break
		}
	}
	if listener == nil {
		return nil, fmt.Errorf("auth: no login callback port available: %w", lastErr)
	}

	server := &LoginServer{
		state:     state,
		port:      port,
		expiresAt: time.Now().Add(opts.Timeout).UTC(),
		listener:  listener,
		result:    make(chan Session, 1),
		cancelled: make(chan struct{}, 1),
	}
	mux := nethttp.NewServeMux()
	mux.HandleFunc("/lovart/auth/session", server.handleSession)
	mux.HandleFunc("/lovart/auth/complete", server.handleComplete)
	mux.HandleFunc("/lovart/auth/cancel", server.handleCancel)
	server.server = &nethttp.Server{Handler: mux}

	go func() {
		<-ctx.Done()
		_ = server.Close(context.Background())
	}()
	go func() {
		if err := server.server.Serve(listener); err != nil && err != nethttp.ErrServerClosed {
			select {
			case server.cancelled <- struct{}{}:
			default:
			}
		}
	}()
	return server, nil
}

// State returns the one-time login state.
func (s *LoginServer) State() string { return s.state }

// Port returns the callback port.
func (s *LoginServer) Port() int { return s.port }

// ExpiresAt returns the login session expiry.
func (s *LoginServer) ExpiresAt() time.Time { return s.expiresAt }

// Result returns the successful auth callback channel.
func (s *LoginServer) Result() <-chan Session { return s.result }

// Cancelled returns a channel closed or signalled when the browser cancels.
func (s *LoginServer) Cancelled() <-chan struct{} { return s.cancelled }

// Close stops the local callback server.
func (s *LoginServer) Close(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	return s.server.Shutdown(ctx)
}

func (s *LoginServer) handleSession(w nethttp.ResponseWriter, r *nethttp.Request) {
	setAuthCORS(w, r)
	if r.Method == nethttp.MethodOptions {
		w.WriteHeader(nethttp.StatusNoContent)
		return
	}
	if r.Method != nethttp.MethodGet {
		w.WriteHeader(nethttp.StatusMethodNotAllowed)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"state":      s.state,
		"expires_at": s.expiresAt.Format(time.RFC3339),
		"version":    1,
	})
}

func (s *LoginServer) handleComplete(w nethttp.ResponseWriter, r *nethttp.Request) {
	setAuthCORS(w, r)
	if r.Method == nethttp.MethodOptions {
		w.WriteHeader(nethttp.StatusNoContent)
		return
	}
	if r.Method != nethttp.MethodPost {
		w.WriteHeader(nethttp.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		State     string `json:"state"`
		Cookie    string `json:"cookie"`
		Token     string `json:"token"`
		CSRF      string `json:"csrf"`
		ProjectID string `json:"project_id"`
		CID       string `json:"cid"`
		Source    string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		nethttp.Error(w, "invalid JSON", nethttp.StatusBadRequest)
		return
	}
	session := Session{
		Cookie:    payload.Cookie,
		Token:     payload.Token,
		CSRF:      payload.CSRF,
		ProjectID: payload.ProjectID,
		CID:       payload.CID,
		Source:    firstNonEmpty(payload.Source, LoginSourceBrowserExtension),
	}
	if session.Cookie == "" && session.Token == "" {
		nethttp.Error(w, "missing cookie or token", nethttp.StatusBadRequest)
		return
	}
	if !s.markUsed(payload.State) {
		nethttp.Error(w, "invalid or used state", nethttp.StatusForbidden)
		return
	}
	select {
	case s.result <- session:
	default:
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *LoginServer) handleCancel(w nethttp.ResponseWriter, r *nethttp.Request) {
	setAuthCORS(w, r)
	if r.Method == nethttp.MethodOptions {
		w.WriteHeader(nethttp.StatusNoContent)
		return
	}
	if r.Method != nethttp.MethodPost {
		w.WriteHeader(nethttp.StatusMethodNotAllowed)
		return
	}
	var payload struct {
		State string `json:"state"`
	}
	_ = json.NewDecoder(r.Body).Decode(&payload)
	if !s.markUsed(payload.State) {
		nethttp.Error(w, "invalid or used state", nethttp.StatusForbidden)
		return
	}
	select {
	case s.cancelled <- struct{}{}:
	default:
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

func (s *LoginServer) markUsed(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used || state == "" || state != s.state || time.Now().After(s.expiresAt) {
		return false
	}
	s.used = true
	return true
}

func setAuthCORS(w nethttp.ResponseWriter, r *nethttp.Request) {
	if origin := r.Header.Get("Origin"); isAllowedAuthOrigin(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Headers", "content-type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Content-Type", "application/json")
}

func isAllowedAuthOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	if strings.HasPrefix(origin, "chrome-extension://") {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "https" {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "www.lovart.ai" || strings.HasSuffix(host, ".lovart.ai")
}

func randomState() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("auth: generate login state: %w", err)
	}
	return hex.EncodeToString(buf[:]), nil
}
