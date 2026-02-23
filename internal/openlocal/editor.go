package openlocal

import "fmt"

// EditorCommand builds the local CLI command to open a remote path in the given editor.
func EditorCommand(editor, sshHost, remotePath string) (bin string, args []string) {
	switch editor {
	case EditorVSCode:
		remote := fmt.Sprintf("ssh-remote+%s", sshHost)
		return "code", []string{"--remote", remote, remotePath}
	default: // zed
		uri := fmt.Sprintf("ssh://%s%s", sshHost, remotePath)
		return "zed", []string{uri}
	}
}

// FallbackCommand returns a human-readable command string for manual execution.
func FallbackCommand(editor, sshHost, remotePath string) string {
	bin, args := EditorCommand(editor, sshHost, remotePath)
	cmd := bin
	for _, a := range args {
		cmd += " " + a
	}
	return cmd
}
