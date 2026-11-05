// Package config resolves the LFED_* environment the lfed CLI exports to a
// plugin process.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/LF-Certification/p3-forge/cli/lfed/sdk/output"
)

// Config is the plugin-side view of the plugin contract (plan section 5).
type Config struct {
	// Context is dev, staging or prod.
	Context string
	// APIBaseURL is the gateway base without a trailing slash.
	APIBaseURL string
	// Output is the renderer the lfed CLI selected with -o.
	Output output.Mode
	// LfedVersion is the semver of the launching lfed binary; empty when
	// the plugin runs standalone.
	LfedVersion string
	// Reexec is the absolute path of the launching lfed binary; empty when
	// the plugin runs standalone.
	Reexec string
	// Debug is true when LFED_DEBUG is set. Plugins SHOULD write diagnostic
	// lines to stderr only when it is true; httpc does so for every request.
	Debug bool
}

// Debug reports whether LFED_DEBUG is set, for code that has no Config.
func Debug() bool { return Getenv("LFED_DEBUG") != "" }

// Getenv is the lookup FromEnv uses. Tests replace it.
var Getenv = os.Getenv

// FromEnv reads LFED_CONTEXT, LFED_API_BASE_URL, LFED_OUTPUT, LFED_CLI_VERSION,
// LFED_REEXEC and LFED_DEBUG. Context and base URL are required: without them the
// plugin cannot reach an API, so the error names the fix. The base URL
// must pass CheckAPIBaseURL.
func FromEnv() (Config, error) {
	cfg := Config{
		Context:     Getenv("LFED_CONTEXT"),
		APIBaseURL:  strings.TrimRight(Getenv("LFED_API_BASE_URL"), "/"),
		LfedVersion: Getenv("LFED_CLI_VERSION"),
		Reexec:      Getenv("LFED_REEXEC"),
		Debug:       Debug(),
	}
	var missing []string
	if cfg.Context == "" {
		missing = append(missing, "LFED_CONTEXT")
	}
	if cfg.APIBaseURL == "" {
		missing = append(missing, "LFED_API_BASE_URL")
	}
	if len(missing) > 0 {
		return Config{}, errors.New(strings.Join(missing, " and ") +
			" not set; run this plugin through \"lfed <plugin>\" or export the LFED_* variables")
	}
	if err := CheckAPIBaseURL(cfg.APIBaseURL); err != nil {
		return Config{}, err
	}
	mode, err := output.ParseMode(Getenv("LFED_OUTPUT"))
	if err != nil {
		return Config{}, err
	}
	cfg.Output = mode
	return cfg, nil
}

// CheckAPIBaseURL accepts an https URL, and an http URL only when the host
// is loopback (localhost, 127.0.0.0/8, ::1). The bearer token goes to
// this URL, so it must not cross the network in clear text.
func CheckAPIBaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err == nil && u.Host != "" {
		switch u.Scheme {
		case "https":
			return nil
		case "http":
			if isLoopback(u.Hostname()) {
				return nil
			}
		}
	}
	return fmt.Errorf("LFED_API_BASE_URL %q must use https (plain http is allowed for loopback only)", raw)
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
