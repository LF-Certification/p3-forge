package credstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

// newTestStore builds a plain-file Store in a temporary directory so tests
// never touch the real OS keychain.
func newTestStore(t *testing.T, dir, context string) Store {
	t.Helper()
	store, err := New(Options{Context: context, Insecure: true, StateDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return store
}

func TestNewRequiresContext(t *testing.T) {
	if _, err := New(Options{Insecure: true, StateDir: t.TempDir()}); err == nil {
		t.Fatal("New without Context: got nil error")
	}
}

func TestStateDirFromEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LFED_STATE_DIR", dir)
	store, err := New(Options{Context: "staging", Insecure: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := store.SaveDeviceState(DeviceState{Insecure: true}); err != nil {
		t.Fatalf("SaveDeviceState: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "state-staging.json")); err != nil {
		t.Fatalf("state file not in LFED_STATE_DIR: %v", err)
	}
}

// TestKeyringPerContext runs the system-store path against go-keyring's
// in-memory mock. Contexts MUST NOT share an item; delete MUST be a
// no-op when nothing is stored.
func TestKeyringPerContext(t *testing.T) {
	keyring.MockInit()
	dir := t.TempDir()
	staging, err := New(Options{Context: "staging", StateDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	prod, err := New(Options{Context: "prod", StateDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := staging.DeleteCredentials(); err != nil {
		t.Fatalf("delete before save: %v", err)
	}
	if err := staging.SaveCredentials(Credentials{RefreshToken: "rt-staging"}); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	if _, err := prod.LoadCredentials(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("prod sees staging credentials: %v", err)
	}
	got, err := staging.LoadCredentials()
	if err != nil || got.RefreshToken != "rt-staging" {
		t.Fatalf("LoadCredentials = %+v, %v", got, err)
	}
	if err := staging.DeleteCredentials(); err != nil {
		t.Fatalf("DeleteCredentials: %v", err)
	}
	if _, err := staging.LoadCredentials(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("after delete: %v", err)
	}
}

func TestCredentialsRoundTrip(t *testing.T) {
	store := newTestStore(t, t.TempDir(), "staging")

	if _, err := store.LoadCredentials(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LoadCredentials before save: got err %v, want ErrNotFound", err)
	}

	want := Credentials{
		RefreshToken:      "refresh-token",
		AccessToken:       "access-token",
		AccessTokenExpiry: time.Now().Add(time.Hour).Truncate(time.Second),
	}
	if err := store.SaveCredentials(want); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	got, err := store.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	// JSON can change the time.Location without changing the instant, so
	// compare the expiry with Equal.
	if got.RefreshToken != want.RefreshToken || got.AccessToken != want.AccessToken || !got.AccessTokenExpiry.Equal(want.AccessTokenExpiry) {
		t.Fatalf("LoadCredentials = %+v, want %+v", got, want)
	}

	if err := store.DeleteCredentials(); err != nil {
		t.Fatalf("DeleteCredentials: %v", err)
	}
	if _, err := store.LoadCredentials(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LoadCredentials after delete: got err %v, want ErrNotFound", err)
	}
	if err := store.DeleteCredentials(); err != nil {
		t.Fatalf("DeleteCredentials on empty store: %v", err)
	}
}

func TestCredentialsFileIsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	store := newTestStore(t, dir, "dev")
	if err := store.SaveCredentials(Credentials{RefreshToken: "rt"}); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "credentials-dev.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("credentials file mode = %o, want 600", perm)
	}
}

func TestContextIsolation(t *testing.T) {
	dir := t.TempDir()
	staging := newTestStore(t, dir, "staging")
	prod := newTestStore(t, dir, "prod")

	if err := prod.SaveCredentials(Credentials{RefreshToken: "prod-rt"}); err != nil {
		t.Fatalf("SaveCredentials prod: %v", err)
	}
	if err := prod.SaveDeviceState(DeviceState{Insecure: true}); err != nil {
		t.Fatalf("SaveDeviceState prod: %v", err)
	}

	if err := staging.SaveCredentials(Credentials{RefreshToken: "staging-rt"}); err != nil {
		t.Fatalf("SaveCredentials staging: %v", err)
	}
	if err := staging.DeleteCredentials(); err != nil {
		t.Fatalf("DeleteCredentials staging: %v", err)
	}
	if err := staging.DeleteDeviceState(); err != nil {
		t.Fatalf("DeleteDeviceState staging: %v", err)
	}

	got, err := prod.LoadCredentials()
	if err != nil || got.RefreshToken != "prod-rt" {
		t.Fatalf("prod credentials after staging logout = %+v, %v; want prod-rt", got, err)
	}
	if _, err := prod.LoadDeviceState(); err != nil {
		t.Fatalf("prod device state after staging logout: %v", err)
	}
}

func TestDeviceStateRoundTrip(t *testing.T) {
	store := newTestStore(t, t.TempDir(), "dev")

	if _, err := store.LoadDeviceState(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LoadDeviceState before save: got err %v, want ErrNotFound", err)
	}

	want := DeviceState{Insecure: true}
	if err := store.SaveDeviceState(want); err != nil {
		t.Fatalf("SaveDeviceState: %v", err)
	}
	got, err := store.LoadDeviceState()
	if err != nil {
		t.Fatalf("LoadDeviceState: %v", err)
	}
	if got != want {
		t.Fatalf("LoadDeviceState = %+v, want %+v", got, want)
	}

	if err := store.DeleteDeviceState(); err != nil {
		t.Fatalf("DeleteDeviceState: %v", err)
	}
	if _, err := store.LoadDeviceState(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LoadDeviceState after delete: got err %v, want ErrNotFound", err)
	}
	if err := store.DeleteDeviceState(); err != nil {
		t.Fatalf("DeleteDeviceState on empty store: %v", err)
	}
}

func TestValidAccessToken(t *testing.T) {
	tests := []struct {
		name  string
		creds Credentials
		want  bool
	}{
		{
			name:  "no access token",
			creds: Credentials{AccessTokenExpiry: time.Now().Add(time.Hour)},
			want:  false,
		},
		{
			name:  "expired",
			creds: Credentials{AccessToken: "tok", AccessTokenExpiry: time.Now().Add(-time.Minute)},
			want:  false,
		},
		{
			name:  "expires within 60 seconds",
			creds: Credentials{AccessToken: "tok", AccessTokenExpiry: time.Now().Add(30 * time.Second)},
			want:  false,
		},
		{
			name:  "valid",
			creds: Credentials{AccessToken: "tok", AccessTokenExpiry: time.Now().Add(time.Hour)},
			want:  true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.creds.ValidAccessToken(); got != tc.want {
				t.Errorf("ValidAccessToken() = %v, want %v", got, tc.want)
			}
		})
	}
}
