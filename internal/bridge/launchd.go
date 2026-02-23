package bridge

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"html/template"
)

const (
	plistLabel = "com.kitmux.bridge"
	plistName  = plistLabel + ".plist"
)

var plistTemplate = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>{{ .Label }}</string>
  <key>ProgramArguments</key>
  <array>
    <string>{{ .Binary }}</string>
    <string>bridge</string>
    <string>serve</string>
    <string>--socket</string>
    <string>{{ .Socket }}</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>{{ .LogDir }}/bridge.log</string>
  <key>StandardErrorPath</key>
  <string>{{ .LogDir }}/bridge.log</string>
</dict>
</plist>
`))

type plistData struct {
	Label  string
	Binary string
	Socket string
	LogDir string
}

// InstallLaunchAgent writes the plist and loads it via launchctl.
func InstallLaunchAgent(socketPath string) error {
	bin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}

	agentsDir := filepath.Join(home, "Library", "LaunchAgents")
	if err := os.MkdirAll(agentsDir, 0o750); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	logDir := filepath.Join(home, "Library", "Logs", "kitmux")
	if err := os.MkdirAll(logDir, 0o750); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	plistPath := filepath.Join(agentsDir, plistName)

	f, err := os.OpenFile(plistPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // path is constructed from known constants
	if err != nil {
		return fmt.Errorf("create plist: %w", err)
	}
	defer func() { _ = f.Close() }()

	data := plistData{
		Label:  plistLabel,
		Binary: bin,
		Socket: socketPath,
		LogDir: logDir,
	}
	if err := plistTemplate.Execute(f, data); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	// Unload first (ignore errors if not loaded)
	_ = exec.Command("launchctl", "unload", plistPath).Run()

	if err := exec.Command("launchctl", "load", plistPath).Run(); err != nil {
		return fmt.Errorf("launchctl load: %w", err)
	}

	fmt.Printf("Installed LaunchAgent: %s\n", plistPath)
	fmt.Printf("Socket: %s\n", socketPath)
	fmt.Printf("Logs: %s/bridge.log\n", logDir)
	return nil
}

// UninstallLaunchAgent removes the plist and unloads it.
func UninstallLaunchAgent() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home: %w", err)
	}
	plistPath := filepath.Join(home, "Library", "LaunchAgents", plistName)

	_ = exec.Command("launchctl", "unload", plistPath).Run()

	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}

	fmt.Println("Uninstalled LaunchAgent")
	return nil
}
