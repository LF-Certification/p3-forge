// Package auth obtains the access token a plugin sends to the API.
package auth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Token runs `<LFED_REEXEC> auth token`, falling back to `lfed` on PATH,
// so a plugin works standalone during development. The lfed CLI refreshes
// the token when necessary and prints the login hint when the user is
// logged out. The child inherits LFED_INSECURE_STORAGE, so it uses the
// credential backend of the parent lfed. The child's stderr is captured
// and returned as the error, with the child's `lfed: ` prefix removed, so
// the plugin prints the message once.
func Token(ctx context.Context) (string, error) {
	exe := os.Getenv("LFED_REEXEC")
	if exe == "" {
		exe = "lfed"
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, exe, "auth", "token")
	cmd.Stdin = os.Stdin
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			msg := strings.TrimPrefix(strings.TrimSpace(stderr.String()), "lfed: ")
			if msg == "" {
				msg = fmt.Sprintf("%s auth token: %s", exe, exitErr)
			}
			return "", errors.New(msg)
		}
		return "", fmt.Errorf("run %s auth token: %w", exe, err)
	}
	tok := strings.TrimSpace(stdout.String())
	if tok == "" {
		return "", fmt.Errorf("%s auth token printed no token; run \"lfed auth login\"", exe)
	}
	return tok, nil
}
