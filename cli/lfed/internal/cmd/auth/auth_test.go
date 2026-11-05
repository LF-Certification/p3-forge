package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/auth"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/contexts"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/credstore"
)

func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

func newFactory(t *testing.T) (*cmdutil.Factory, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	t.Setenv("LFED_STATE_DIR", t.TempDir())
	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	f := &cmdutil.Factory{
		IOStreams:       cmdutil.IOStreams{In: strings.NewReader(""), Out: out, ErrOut: errOut},
		OutputFlag:      "table",
		InsecureStorage: true,
		Context:         func() (contexts.Context, error) { return contexts.Lookup("staging") },
	}
	f.Token = auth.TokenResolver(f)
	return f, out, errOut
}

func seedLogin(t *testing.T, accessToken string) {
	t.Helper()
	store, err := credstore.New(credstore.Options{Context: "staging", Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCredentials(credstore.Credentials{RefreshToken: "rt", AccessToken: accessToken, AccessTokenExpiry: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveDeviceState(credstore.DeviceState{Insecure: true}); err != nil {
		t.Fatal(err)
	}
}

func jwt(t *testing.T, payload map[string]any) string {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	seg := base64.RawURLEncoding.EncodeToString
	return seg([]byte(`{"alg":"none"}`)) + "." + seg(b) + ".sig"
}

func run(t *testing.T, f *cmdutil.Factory, args ...string) error {
	t.Helper()
	cmd := NewCmd(f)

	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	cmd.SetArgs(args)
	cmd.SetOut(f.IOStreams.Out)
	cmd.SetErr(f.IOStreams.ErrOut)
	return cmd.ExecuteContext(context.Background())
}

func TestStatusNotLoggedIn(t *testing.T) {
	f, out, _ := newFactory(t)
	err := run(t, f, "status")
	if err == nil || err.Error() != `not logged in; run "lfed auth login"` {
		t.Fatalf("err = %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want empty", out.String())
	}
}

func TestStatusJSON(t *testing.T) {
	f, out, _ := newFactory(t)
	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	seedLogin(t, jwt(t, map[string]any{auth.UsernameClaim: "jdoe", "iss": "https://issuer.test/", "exp": exp.Unix()}))
	f.OutputFlag = "json"
	if err := run(t, f, "status"); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Context  string    `json:"context"`
		Username string    `json:"username"`
		Issuer   string    `json:"issuer"`
		Expires  time.Time `json:"expires"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", out.String(), err)
	}
	if got.Context != "staging" || got.Username != "jdoe" || got.Issuer != "https://issuer.test/" || !got.Expires.Equal(exp) {
		t.Errorf("status = %+v", got)
	}
}

func TestStatusTable(t *testing.T) {
	f, out, _ := newFactory(t)
	seedLogin(t, jwt(t, map[string]any{auth.UsernameClaim: "jdoe", "exp": time.Now().Add(time.Hour).Unix()}))
	if err := run(t, f, "status"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "USERNAME") || !strings.Contains(out.String(), "jdoe") || !strings.Contains(out.String(), "staging") {
		t.Errorf("table = %q", out.String())
	}
}

func TestTokenPrintsOnlyToken(t *testing.T) {
	f, out, errOut := newFactory(t)
	seedLogin(t, "opaque-token")
	if err := run(t, f, "token"); err != nil {
		t.Fatal(err)
	}
	if out.String() != "opaque-token\n" {
		t.Errorf("stdout = %q", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("stderr = %q, want empty", errOut.String())
	}
}

func TestLoginWithTokenThenLogout(t *testing.T) {
	f, out, errOut := newFactory(t)
	f.IOStreams.In = strings.NewReader("  my-refresh-token \n")
	if err := run(t, f, "login", "--with-token"); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || !strings.Contains(errOut.String(), "Logged in to staging") {
		t.Errorf("stdout = %q, stderr = %q", out.String(), errOut.String())
	}
	store, err := credstore.New(credstore.Options{Context: "staging", Insecure: true})
	if err != nil {
		t.Fatal(err)
	}
	creds, err := store.LoadCredentials()
	if err != nil || creds.RefreshToken != "my-refresh-token" {
		t.Fatalf("creds = %+v, %v", creds, err)
	}

	errOut.Reset()
	if err := run(t, f, "logout"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "Logged out of staging") {
		t.Errorf("stderr = %q", errOut.String())
	}
	if err := run(t, f, "token"); err == nil || err.Error() != `not logged in; run "lfed auth login"` {
		t.Fatalf("token after logout: %v", err)
	}
}

func TestLoginWithTokenEmptyStdin(t *testing.T) {
	f, _, _ := newFactory(t)
	if err := run(t, f, "login", "--with-token"); err == nil || !strings.Contains(err.Error(), "no refresh token provided") {
		t.Fatalf("err = %v", err)
	}
}

func TestLogoutWithoutInsecureFlag(t *testing.T) {
	f, _, errOut := newFactory(t)
	stateDir := os.Getenv("LFED_STATE_DIR")
	f.IOStreams.In = strings.NewReader("my-refresh-token\n")
	if err := run(t, f, "login", "--with-token"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"credentials-staging.json", "state-staging.json"} {
		if _, err := os.Stat(filepath.Join(stateDir, name)); err != nil {
			t.Fatalf("%s after login: %v", name, err)
		}
	}
	f.InsecureStorage = false
	errOut.Reset()
	if err := run(t, f, "logout"); err != nil {
		t.Fatal(err)
	}
	if errOut.String() != "Logged out of staging.\n" {
		t.Errorf("stderr = %q", errOut.String())
	}
	for _, name := range []string{"credentials-staging.json", "state-staging.json"} {
		if _, err := os.Stat(filepath.Join(stateDir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s after logout: %v", name, err)
		}
	}
}

func TestTokenInsecureStorageFromEnv(t *testing.T) {
	for _, tc := range []struct {
		env     string
		wantErr string
	}{
		{env: "1"},
		{env: "", wantErr: "pass --insecure-storage"},
	} {
		t.Run("LFED_INSECURE_STORAGE="+tc.env, func(t *testing.T) {
			f, out, _ := newFactory(t)
			seedLogin(t, "opaque-token")
			f.InsecureStorage = false
			t.Setenv("LFED_INSECURE_STORAGE", tc.env)
			err := run(t, f, "token")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || out.String() != "opaque-token\n" {
				t.Fatalf("stdout = %q, err = %v", out.String(), err)
			}
		})
	}
}

func TestLoginIgnoresInsecureStorageEnv(t *testing.T) {
	f, _, _ := newFactory(t)
	f.InsecureStorage = false
	t.Setenv("LFED_INSECURE_STORAGE", "1")
	f.IOStreams.In = strings.NewReader("rt\n")
	if err := run(t, f, "login", "--with-token"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(os.Getenv("LFED_STATE_DIR"), "credentials-staging.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("credentials file after login without flag: %v", err)
	}
	if _, err := keyring.Get("lfed", "credentials/staging"); err != nil {
		t.Errorf("keyring item: %v", err)
	}
}

func TestAuthUnknownSubcommand(t *testing.T) {
	f, _, _ := newFactory(t)
	if err := run(t, f, "nosuchcmd"); err == nil || !strings.Contains(err.Error(), `unknown command "nosuchcmd"`) {
		t.Fatalf("err = %v", err)
	}
}
