package openlocal

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Request is the JSON payload sent to the bridge.
type Request struct {
	Editor string `json:"editor"`
	Host   string `json:"host"`
	Path   string `json:"path"`
}

// Response is the JSON payload returned by the bridge.
type Response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// SendOpenRequest sends an open-editor request to the bridge socket.
// Returns nil on success, an error otherwise.
func SendOpenRequest(socketPath string, req Request) error {
	conn, err := net.DialTimeout("unix", socketPath, 3*time.Second)
	if err != nil {
		return fmt.Errorf("bridge not reachable: %w", err)
	}
	defer func() { _ = conn.Close() }()

	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if !resp.OK {
		return fmt.Errorf("bridge error: %s", resp.Error)
	}
	return nil
}
