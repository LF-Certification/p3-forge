package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/contexts"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/credstore"
)

func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

type tokenReply struct {
	status int
	body   map[string]any
}

func oauthErr(code string) tokenReply {
	return tokenReply{status: http.StatusBadRequest, body: map[string]any{"error": code, "error_description": code}}
}

func oauthOK(body map[string]any) tokenReply {
	return tokenReply{status: http.StatusOK, body: body}
}

type fakeAuth0 struct {
	t       *testing.T
	mu      sync.Mutex
	replies []tokenReply

	deviceReqs []map[string]string
	tokenReqs  []map[string]string
	expiresIn  int
}

func newFakeAuth0(t *testing.T, replies ...tokenReply) (*fakeAuth0, *httptest.Server) {
	t.Helper()
	f := &fakeAuth0{t: t, replies: replies, expiresIn: 60}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeAuth0) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		f.t.Errorf("parse form: %v", err)
	}
	form := map[string]string{}
	for k := range r.PostForm {
		form[k] = r.PostForm.Get(k)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/oauth/device/code":
		f.deviceReqs = append(f.deviceReqs, form)
		json.NewEncoder(w).Encode(map[string]any{
			"device_code":               "dev-code",
			"user_code":                 "ABCD-EFGH",
			"verification_uri":          "https://example.test/activate",
			"verification_uri_complete": "https://example.test/activate?user_code=ABCD-EFGH",
			"expires_in":                f.expiresIn,
			"interval":                  1,
		})
	case "/oauth/token":
		f.tokenReqs = append(f.tokenReqs, form)
		reply := f.replies[0]
		if len(f.replies) > 1 {
			f.replies = f.replies[1:]
		}
		w.WriteHeader(reply.status)
		json.NewEncoder(w).Encode(reply.body)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeAuth0) tokenRequests() []map[string]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]map[string]string(nil), f.tokenReqs...)
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

func testContext(t *testing.T, srv *httptest.Server) contexts.Context {
	t.Helper()
	c, err := contexts.Lookup("staging")
	if err != nil {
		t.Fatal(err)
	}
	prev := Auth0BaseURL
	Auth0BaseURL = func(contexts.Context) string { return srv.URL }
	t.Cleanup(func() { Auth0BaseURL = prev })
	return c
}

func testStore(t *testing.T) credstore.Store {
	t.Helper()
	store, err := credstore.New(credstore.Options{Context: "staging", Insecure: true, StateDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestDeviceLoginPendingThenSuccess(t *testing.T) {
	fake, srv := newFakeAuth0(t,
		oauthErr("authorization_pending"),
		oauthOK(map[string]any{
			"access_token":  "at-1",
			"refresh_token": "rt-1",
			"expires_in":    3600,
			"token_type":    "Bearer",
			"id_token":      jwt(t, map[string]any{idTokenUsernameClaim: "jdoe", "email": "jdoe@example.test"}),
		}),
	)
	c := testContext(t, srv)
	store := testStore(t)
	var out bytes.Buffer

	identity, err := DeviceLogin(context.Background(), c, store, LoginOptions{Out: &out, Insecure: true})
	if err != nil {
		t.Fatalf("DeviceLogin: %v", err)
	}
	if identity != "jdoe (jdoe@example.test)" {
		t.Errorf("identity = %q", identity)
	}
	if !strings.Contains(out.String(), "ABCD-EFGH") || !strings.Contains(out.String(), "https://example.test/activate?user_code=ABCD-EFGH") {
		t.Errorf("instructions missing code or URL:\n%s", out.String())
	}

	dev := fake.deviceReqs[0]
	if dev["audience"] != c.Audience || dev["client_id"] != c.ClientID || !strings.Contains(dev["scope"], "offline_access") {
		t.Errorf("device request = %v", dev)
	}
	reqs := fake.tokenRequests()
	if len(reqs) != 2 {
		t.Fatalf("token requests = %d, want 2", len(reqs))
	}
	if reqs[0]["grant_type"] != "urn:ietf:params:oauth:grant-type:device_code" || reqs[0]["device_code"] != "dev-code" || reqs[0]["client_id"] != c.ClientID {
		t.Errorf("token request = %v", reqs[0])
	}

	creds, err := store.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if creds.RefreshToken != "rt-1" || creds.AccessToken != "at-1" || !creds.ValidAccessToken() {
		t.Errorf("persisted creds = %+v", creds)
	}
	state, err := store.LoadDeviceState()
	if err != nil || !state.Insecure {
		t.Errorf("state = %+v, %v; want Insecure", state, err)
	}
}

func TestDeviceLoginSlowDown(t *testing.T) {
	fake, srv := newFakeAuth0(t,
		oauthErr("slow_down"),
		oauthOK(map[string]any{"access_token": "at", "refresh_token": "rt", "expires_in": 3600, "token_type": "Bearer"}),
	)
	c := testContext(t, srv)
	start := time.Now()
	if _, err := DeviceLogin(context.Background(), c, testStore(t), LoginOptions{Out: &bytes.Buffer{}, Insecure: true}); err != nil {
		t.Fatalf("DeviceLogin: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 6*time.Second {
		t.Errorf("polling did not back off after slow_down: %v", elapsed)
	}
	if n := len(fake.tokenRequests()); n != 2 {
		t.Errorf("token requests = %d, want 2", n)
	}
}

func TestDeviceLoginExpired(t *testing.T) {
	_, srv := newFakeAuth0(t, oauthErr("expired_token"))
	c := testContext(t, srv)
	store := testStore(t)
	_, err := DeviceLogin(context.Background(), c, store, LoginOptions{Out: &bytes.Buffer{}, Insecure: true})
	if err == nil || err.Error() != "timed out waiting for login" {
		t.Fatalf("err = %v", err)
	}
	if _, err := store.LoadCredentials(); !errors.Is(err, credstore.ErrNotFound) {
		t.Errorf("credentials stored after failed login: %v", err)
	}
}

func TestDeviceLoginDenied(t *testing.T) {
	_, srv := newFakeAuth0(t, oauthErr("access_denied"))
	c := testContext(t, srv)
	_, err := DeviceLogin(context.Background(), c, testStore(t), LoginOptions{Out: &bytes.Buffer{}, Insecure: true})
	if err == nil || err.Error() != "login was denied" {
		t.Fatalf("err = %v", err)
	}
}

func TestDeviceLoginWithoutRefreshToken(t *testing.T) {
	_, srv := newFakeAuth0(t, oauthOK(map[string]any{"access_token": "at", "expires_in": 3600, "token_type": "Bearer"}))
	c := testContext(t, srv)
	store := testStore(t)
	_, err := DeviceLogin(context.Background(), c, store, LoginOptions{Out: &bytes.Buffer{}, Insecure: true})
	if err == nil || !strings.Contains(err.Error(), "did not return a refresh token") {
		t.Fatalf("err = %v", err)
	}
	if _, err := store.LoadCredentials(); !errors.Is(err, credstore.ErrNotFound) {
		t.Errorf("credentials stored without refresh token: %v", err)
	}
}

func TestTokenCachedNoNetwork(t *testing.T) {
	fake, srv := newFakeAuth0(t, oauthErr("server_error"))
	c := testContext(t, srv)
	store := testStore(t)
	if err := persistLogin(context.Background(), store, credstore.Credentials{RefreshToken: "rt", AccessToken: "cached", AccessTokenExpiry: time.Now().Add(time.Hour)}, true); err != nil {
		t.Fatal(err)
	}
	got, err := Token(context.Background(), c, store, true)
	if err != nil || got != "cached" {
		t.Fatalf("Token = %q, %v", got, err)
	}
	if n := len(fake.tokenRequests()); n != 0 {
		t.Errorf("token requests = %d, want 0", n)
	}
}

func TestTokenRefreshesExpired(t *testing.T) {
	fake, srv := newFakeAuth0(t, oauthOK(map[string]any{"access_token": "at-2", "refresh_token": "rt-2", "expires_in": 3600, "token_type": "Bearer"}))
	c := testContext(t, srv)
	store := testStore(t)

	if err := persistLogin(context.Background(), store, credstore.Credentials{RefreshToken: "rt-1", AccessToken: "at-1", AccessTokenExpiry: time.Now().Add(30 * time.Second)}, true); err != nil {
		t.Fatal(err)
	}
	got, err := Token(context.Background(), c, store, true)
	if err != nil || got != "at-2" {
		t.Fatalf("Token = %q, %v", got, err)
	}
	reqs := fake.tokenRequests()
	if len(reqs) != 1 || reqs[0]["grant_type"] != "refresh_token" || reqs[0]["refresh_token"] != "rt-1" || reqs[0]["client_id"] != c.ClientID {
		t.Errorf("refresh request = %v", reqs)
	}
	creds, err := store.LoadCredentials()
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessToken != "at-2" || creds.RefreshToken != "rt-2" || !creds.ValidAccessToken() {
		t.Errorf("persisted creds = %+v", creds)
	}
}

func TestTokenRefreshKeepsRefreshTokenWhenNotRotated(t *testing.T) {
	_, srv := newFakeAuth0(t, oauthOK(map[string]any{"access_token": "at-2", "expires_in": 3600, "token_type": "Bearer"}))
	c := testContext(t, srv)
	store := testStore(t)
	if err := persistLogin(context.Background(), store, credstore.Credentials{RefreshToken: "rt-1"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := Token(context.Background(), c, store, true); err != nil {
		t.Fatal(err)
	}
	creds, _ := store.LoadCredentials()
	if creds.RefreshToken != "rt-1" {
		t.Errorf("refresh token = %q, want rt-1", creds.RefreshToken)
	}
}

func TestTokenErrors(t *testing.T) {
	_, srv := newFakeAuth0(t, oauthErr("invalid_grant"))
	c := testContext(t, srv)

	t.Run("not logged in", func(t *testing.T) {
		_, err := Token(context.Background(), c, testStore(t), true)
		if !errors.Is(err, ErrNotLoggedIn) || err.Error() != `not logged in; run "lfed auth login"` {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("invalid_grant", func(t *testing.T) {
		store := testStore(t)
		if err := persistLogin(context.Background(), store, credstore.Credentials{RefreshToken: "revoked"}, true); err != nil {
			t.Fatal(err)
		}
		_, err := Token(context.Background(), c, store, true)
		if !errors.Is(err, ErrSessionExpired) || err.Error() != `session expired or revoked; run "lfed auth login"` {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("backend mismatch", func(t *testing.T) {
		store := testStore(t)
		if err := persistLogin(context.Background(), store, credstore.Credentials{RefreshToken: "rt"}, true); err != nil {
			t.Fatal(err)
		}
		_, err := Token(context.Background(), c, store, false)
		if err == nil || !strings.Contains(err.Error(), "pass --insecure-storage") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestLogoutRemovesEverything(t *testing.T) {
	store := testStore(t)
	if err := persistLogin(context.Background(), store, credstore.Credentials{RefreshToken: "rt"}, true); err != nil {
		t.Fatal(err)
	}
	if _, err := Logout(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadCredentials(); !errors.Is(err, credstore.ErrNotFound) {
		t.Errorf("credentials after logout: %v", err)
	}
	if _, err := store.LoadDeviceState(); !errors.Is(err, credstore.ErrNotFound) {
		t.Errorf("state after logout: %v", err)
	}

	if _, err := Logout(context.Background(), store); err != nil {
		t.Errorf("second logout: %v", err)
	}
}

func TestLogoutIgnoresFlag(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	fileStore := mustStore(t, dir, true)
	if err := TokenLogin(context.Background(), fileStore, "rt", true); err != nil {
		t.Fatal(err)
	}
	warning, err := Logout(context.Background(), mustStore(t, dir, false))
	if err != nil || warning != "" {
		t.Fatalf("Logout = %q, %v", warning, err)
	}
	for _, name := range []string{"credentials-staging.json", "state-staging.json"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s after logout: %v", name, err)
		}
	}
}

func TestLogoutWithoutStateRemovesFile(t *testing.T) {
	dir := t.TempDir()
	store := mustStore(t, dir, true)
	if err := store.SaveCredentials(credstore.Credentials{RefreshToken: "rt"}); err != nil {
		t.Fatal(err)
	}
	if _, err := Logout(context.Background(), store); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "credentials-staging.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("credentials file after logout: %v", err)
	}
}

func TestLogoutOtherBackendFailureIsWarning(t *testing.T) {
	keyring.MockInitWithError(errors.New("no D-Bus session"))
	t.Cleanup(keyring.MockInit)
	store := mustStore(t, t.TempDir(), true)
	if err := TokenLogin(context.Background(), store, "rt", true); err != nil {
		t.Fatal(err)
	}
	warning, err := Logout(context.Background(), store)
	if err != nil || !strings.Contains(warning, "no D-Bus session") {
		t.Fatalf("Logout = %q, %v", warning, err)
	}
	if _, err := store.LoadDeviceState(); !errors.Is(err, credstore.ErrNotFound) {
		t.Errorf("state after logout: %v", err)
	}
}

func TestBackendSwitchRemovesOldSecret(t *testing.T) {
	t.Run("keyring to file", func(t *testing.T) {
		keyring.MockInit()
		dir := t.TempDir()
		if err := TokenLogin(context.Background(), mustStore(t, dir, false), "rt-keyring", false); err != nil {
			t.Fatal(err)
		}
		if err := TokenLogin(context.Background(), mustStore(t, dir, true), "rt-file", true); err != nil {
			t.Fatal(err)
		}
		if _, err := keyring.Get("lfed", "credentials/staging"); !errors.Is(err, keyring.ErrNotFound) {
			t.Errorf("keyring item after switch: %v", err)
		}
	})
	t.Run("file to keyring", func(t *testing.T) {
		keyring.MockInit()
		dir := t.TempDir()
		if err := TokenLogin(context.Background(), mustStore(t, dir, true), "rt-file", true); err != nil {
			t.Fatal(err)
		}
		if err := TokenLogin(context.Background(), mustStore(t, dir, false), "rt-keyring", false); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "credentials-staging.json")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("credentials file after switch: %v", err)
		}
		if _, err := keyring.Get("lfed", "credentials/staging"); err != nil {
			t.Errorf("keyring item after switch: %v", err)
		}
	})
}

func TestTokenConcurrentRefreshOnce(t *testing.T) {
	fake, srv := newFakeAuth0(t, oauthOK(map[string]any{"access_token": "at-2", "refresh_token": "rt-2", "expires_in": 3600, "token_type": "Bearer"}))
	c := testContext(t, srv)
	dir := t.TempDir()
	if err := persistLogin(context.Background(), mustStore(t, dir, true), credstore.Credentials{RefreshToken: "rt-1"}, true); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	tokens := make([]string, 2)
	errs := make([]error, 2)
	for i := range 2 {

		store := mustStore(t, dir, true)
		wg.Go(func() { tokens[i], errs[i] = Token(context.Background(), c, store, true) })
	}
	wg.Wait()
	for i := range 2 {
		if errs[i] != nil || tokens[i] != "at-2" {
			t.Errorf("Token %d = %q, %v", i, tokens[i], errs[i])
		}
	}
	if n := len(fake.tokenRequests()); n != 1 {
		t.Errorf("refresh requests = %d, want 1", n)
	}
}

func TestLockWaitEndsWithContext(t *testing.T) {
	dir := t.TempDir()
	if err := persistLogin(context.Background(), mustStore(t, dir, true), credstore.Credentials{RefreshToken: "rt"}, true); err != nil {
		t.Fatal(err)
	}
	unlock, err := mustStore(t, dir, true).Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Token(ctx, contexts.Context{}, mustStore(t, dir, true), true); !errors.Is(err, context.Canceled) {
		t.Fatalf("Token with a held lock and a canceled context: err = %v", err)
	}
}

func TestTokenHTTPTimeout(t *testing.T) {
	ctx := withHTTPClient(context.Background())
	client, ok := ctx.Value(oauth2.HTTPClient).(*http.Client)
	if !ok || client.Timeout != httpTimeout || client.Transport != nil {
		t.Fatalf("oauth2 client = %#v", ctx.Value(oauth2.HTTPClient))
	}
}

func mustStore(t *testing.T, dir string, insecure bool) credstore.Store {
	t.Helper()
	store, err := credstore.New(credstore.Options{Context: "staging", Insecure: insecure, StateDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestDecodeClaims(t *testing.T) {
	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	tok := jwt(t, map[string]any{UsernameClaim: "jdoe", "iss": "https://issuer.test/", "sub": "auth0|1", "exp": exp.Unix()})
	claims, err := DecodeClaims(tok)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Username != "jdoe" || claims.Issuer != "https://issuer.test/" || claims.Subject != "auth0|1" || !claims.Expiry.Equal(exp) {
		t.Errorf("claims = %+v", claims)
	}
	if _, err := DecodeClaims("opaque"); err == nil {
		t.Error("DecodeClaims(opaque): want error")
	}
}

func TestTerminalSafeText(t *testing.T) {
	if got := terminalSafeText("ABCD-EFGH"); got != "ABCD-EFGH" {
		t.Errorf("plain = %q", got)
	}
	if got := terminalSafeText("A\x1b[31mB"); got != `"A\x1b[31mB"` {
		t.Errorf("escaped = %q", got)
	}
}

func TestValidateBrowserURL(t *testing.T) {
	for _, bad := range []string{"http://example.test", "file:///etc/passwd", "https://", "mailto:x@y", "https://example.test/\x07"} {
		if err := validateBrowserURL(bad); err == nil {
			t.Errorf("validateBrowserURL(%q): want error", bad)
		}
	}
	if err := validateBrowserURL("https://example.test/activate?user_code=X"); err != nil {
		t.Errorf("valid URL rejected: %v", err)
	}
}
