package bridge

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/miltonparedes/kitmux/internal/openlocal"
)

var allowedEditors = map[string]bool{
	openlocal.EditorZed:    true,
	openlocal.EditorVSCode: true,
}

// Serve starts the bridge listener on the given Unix socket path.
func Serve(socketPath string) error {
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove old socket: %w", err)
	}

	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer func() { _ = ln.Close() }()

	if err := os.Chmod(socketPath, 0o600); err != nil {
		return fmt.Errorf("chmod socket: %w", err)
	}

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		_ = ln.Close()
	}()

	log.Printf("bridge: listening on %s", socketPath)

	for {
		conn, err := ln.Accept()
		if err != nil {
			return nil // listener closed
		}
		go handleConn(conn)
	}
}

func handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	var req openlocal.Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		writeError(conn, "invalid request")
		return
	}

	if !allowedEditors[req.Editor] {
		writeError(conn, fmt.Sprintf("unsupported editor %q", req.Editor))
		return
	}
	if req.Host == "" || req.Path == "" {
		writeError(conn, "host and path are required")
		return
	}
	if len(req.Host) > 256 || len(req.Path) > 4096 {
		writeError(conn, "field too long")
		return
	}

	bin, args := openlocal.EditorCommand(req.Editor, req.Host, req.Path)
	cmd := exec.Command(bin, args...)
	if err := cmd.Start(); err != nil {
		writeError(conn, fmt.Sprintf("launch %s: %v", bin, err))
		return
	}

	log.Printf("bridge: opened %s %v (pid %d)", bin, args, cmd.Process.Pid)
	writeOK(conn)
}

func writeOK(conn net.Conn) {
	_ = json.NewEncoder(conn).Encode(openlocal.Response{OK: true})
}

func writeError(conn net.Conn, msg string) {
	_ = json.NewEncoder(conn).Encode(openlocal.Response{OK: false, Error: msg})
}
