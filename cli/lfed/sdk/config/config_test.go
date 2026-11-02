package config

import (
	"strings"
	"testing"

	"github.com/LF-Certification/p3-forge/cli/lfed/sdk/output"
)

func stubEnv(t *testing.T, env map[string]string) {
	t.Helper()
	old := Getenv
	Getenv = func(k string) string { return env[k] }
	t.Cleanup(func() { Getenv = old })
}

func TestFromEnvFull(t *testing.T) {
	stubEnv(t, map[string]string{
		"LFED_CONTEXT":      "staging",
		"LFED_API_BASE_URL": "https://api.example.test/",
		"LFED_OUTPUT":       "json",
		"LFED_CLI_VERSION":  "0.1.0",
		"LFED_REEXEC":       "/usr/local/bin/lfed",
	})
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIBaseURL != "https://api.example.test" {
		t.Errorf("trailing slash kept: %q", cfg.APIBaseURL)
	}
	if cfg.Output != output.JSON || cfg.Context != "staging" || cfg.LfedVersion != "0.1.0" || cfg.Reexec != "/usr/local/bin/lfed" {
		t.Errorf("unexpected config %+v", cfg)
	}
}

func TestFromEnvDefaultsOutputToTable(t *testing.T) {
	stubEnv(t, map[string]string{"LFED_CONTEXT": "dev", "LFED_API_BASE_URL": "http://x"})
	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Output != output.Table {
		t.Errorf("got %q", cfg.Output)
	}
}

func TestFromEnvMissing(t *testing.T) {
	stubEnv(t, map[string]string{"LFED_CONTEXT": "dev"})
	_, err := FromEnv()
	if err == nil || !strings.Contains(err.Error(), "LFED_API_BASE_URL") || !strings.Contains(err.Error(), "lfed <plugin>") {
		t.Fatalf("error should name the variable and the fix: %v", err)
	}
	stubEnv(t, map[string]string{})
	_, err = FromEnv()
	if err == nil || !strings.Contains(err.Error(), "LFED_CONTEXT and LFED_API_BASE_URL") {
		t.Fatalf("got %v", err)
	}
}

func TestFromEnvRejectsBadOutput(t *testing.T) {
	stubEnv(t, map[string]string{"LFED_CONTEXT": "dev", "LFED_API_BASE_URL": "http://x", "LFED_OUTPUT": "xml"})
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected error")
	}
}
