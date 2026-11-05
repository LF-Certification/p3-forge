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

type Config struct {
	Context string

	APIBaseURL string

	Output output.Mode

	LfedVersion string

	Reexec string

	Debug bool
}

func Debug() bool { return Getenv("LFED_DEBUG") != "" }

var Getenv = os.Getenv

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
