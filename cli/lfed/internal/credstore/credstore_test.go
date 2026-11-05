package credstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	if err := staging.DeleteCredentials(false); err != nil {
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
	if err := staging.DeleteCredentials(false); err != nil {
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

	if err := store.DeleteCredentials(true); err != nil {
		t.Fatalf("DeleteCredentials: %v", err)
	}
	if _, err := store.LoadCredentials(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("LoadCredentials after delete: got err %v, want ErrNotFound", err)
	}
	if err := store.DeleteCredentials(true); err != nil {
		t.Fatalf("DeleteCredentials on empty store: %v", err)
	}
}

func TestCredentialsFileIsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials-dev.json")
	// A wider file from an older release is narrowed on the next write.
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	store := newTestStore(t, dir, "dev")
	if err := store.SaveCredentials(Credentials{RefreshToken: "rt"}); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	for p, want := range map[string]os.FileMode{path: 0o600, dir: 0o700} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != want {
			t.Errorf("%s mode = %o, want %o", p, perm, want)
		}
	}
}

// TestWriteOwnerOnlyFileFailureLeavesNoTemp makes the final rename fail:
// the target is a non-empty directory. The temporary file MUST be removed.
func TestWriteOwnerOnlyFileFailureLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.MkdirAll(filepath.Join(target, "child"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := WriteOwnerOnlyFile(target, []byte("data")); err == nil {
		t.Fatal("WriteOwnerOnlyFile over a directory: got nil error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("temporary file left behind: %s", e.Name())
		}
	}
}

// TestWriteOwnerOnlyFileReplaces checks that a second write replaces the
// content and leaves only the target file.
func TestWriteOwnerOnlyFileReplaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.json")
	for _, data := range []string{"first, longer content", "second"} {
		if err := WriteOwnerOnlyFile(path, []byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "second" {
		t.Fatalf("content = %q, %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("dir entries = %v, %v", entries, err)
	}
}

// TestLock checks that a held lock blocks a second Lock call until its
// context ends, and that unlock releases it.
func TestLock(t *testing.T) {
	dir := t.TempDir()
	first := newTestStore(t, dir, "dev")
	second := newTestStore(t, dir, "dev")
	unlock, err := first.Lock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := second.Lock(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Lock while held: err = %v, want context.Canceled", err)
	}
	// Another context has its own lock.
	unlockProd, err := newTestStore(t, dir, "prod").Lock(context.Background())
	if err != nil {
		t.Fatalf("Lock prod: %v", err)
	}
	unlockProd()
	unlock()
	unlock2, err := second.Lock(context.Background())
	if err != nil {
		t.Fatalf("Lock after unlock: %v", err)
	}
	unlock2()
	if _, err := os.Stat(filepath.Join(dir, "lock-dev")); err != nil {
		t.Errorf("lock file: %v", err)
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
	if err := staging.DeleteCredentials(true); err != nil {
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
