// Package notifysock provides a Unix socket server for communicating
// with the native notification helper process.
//
// Protocol (newline-delimited JSON):
//
//	App → Helper: {"type":"notify","id":"uuid","title":"...","subtitle":"...","body":"...","action":"proposals"}
//	Helper → App: {"type":"clicked","id":"uuid","action":"proposals"}
package notifysock

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
)

// Message is the JSON envelope for socket communication.
type Message struct {
	Type     string `json:"type"`               // "notify" or "clicked"
	ID       string `json:"id,omitempty"`        // Notification ID
	Title    string `json:"title,omitempty"`
	Subtitle string `json:"subtitle,omitempty"`
	Body     string `json:"body,omitempty"`
	Action   string `json:"action,omitempty"`    // Tab to open on click: "proposals", "dashboard", etc.
}

// Server manages the Unix socket for notification IPC.
type Server struct {
	sockPath string
	listener net.Listener
	conn     net.Conn
	mu       sync.Mutex
	// OnAction is called when the user clicks a notification.
	OnAction func(action string)
}

// NewServer creates a new notification socket server.
// Socket is placed in /tmp to avoid spaces in path (macOS "Application Support"
// causes argument splitting issues when passing to helper process).
func NewServer(dataDir string) *Server {
	return &Server{
		sockPath: fmt.Sprintf("/tmp/spa-notify-%d.sock", os.Getpid()),
	}
}

// Start begins listening on the Unix socket.
func (s *Server) Start(ctx context.Context) error {
	// Remove stale socket file
	os.Remove(s.sockPath)

	listener, err := net.Listen("unix", s.sockPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.sockPath, err)
	}
	s.listener = listener

	// Accept connections in background
	go s.acceptLoop(ctx)

	return nil
}

// Send sends a notification request to the connected helper.
func (s *Server) Send(msg Message) error {
	s.mu.Lock()
	conn := s.conn
	s.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("no helper connected")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, err = conn.Write(data)
	return err
}

// Notify sends a notification to the helper.
func (s *Server) Notify(id, title, subtitle, body, action string) error {
	return s.Send(Message{
		Type:     "notify",
		ID:       id,
		Title:    title,
		Subtitle: subtitle,
		Body:     body,
		Action:   action,
	})
}

// Stop shuts down the server and cleans up.
func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.Lock()
	if s.conn != nil {
		s.conn.Close()
	}
	s.mu.Unlock()
	os.Remove(s.sockPath)
}

// Connected returns whether a helper is connected.
func (s *Server) Connected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conn != nil
}

// SocketPath returns the path to the Unix socket.
func (s *Server) SocketPath() string {
	return s.sockPath
}

func (s *Server) acceptLoop(ctx context.Context) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

		s.mu.Lock()
		if s.conn != nil {
			s.conn.Close() // Replace old connection
		}
		s.conn = conn
		s.mu.Unlock()

		go s.readLoop(ctx, conn)
	}
}

func (s *Server) readLoop(ctx context.Context, conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var msg Message
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		if msg.Type == "clicked" && s.OnAction != nil {
			s.OnAction(msg.Action)
		}
	}
}
