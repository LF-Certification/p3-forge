package credstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gofrs/flock"
	"github.com/zalando/go-keyring"
)

var ErrNotFound = errors.New("credstore: not found")

const serviceName = "lfed"

type Credentials struct {
	RefreshToken      string    `json:"refresh_token"`
	AccessToken       string    `json:"access_token,omitempty"`
	AccessTokenExpiry time.Time `json:"access_token_expiry,omitempty"`
}

const refreshLeeway = 60 * time.Second

func (c Credentials) ValidAccessToken() bool {
	return c.AccessToken != "" && time.Now().Add(refreshLeeway).Before(c.AccessTokenExpiry)
}

type DeviceState struct {
	Insecure bool `json:"insecure,omitempty"`
}

type Store interface {
	SaveCredentials(creds Credentials) error

	LoadCredentials() (Credentials, error)

	DeleteCredentials(insecure bool) error

	SaveDeviceState(state DeviceState) error

	LoadDeviceState() (DeviceState, error)

	DeleteDeviceState() error

	Lock(ctx context.Context) (unlock func(), err error)
}

const LockTimeout = 30 * time.Second

const lockRetry = 100 * time.Millisecond

type Options struct {
	Context string

	Insecure bool

	StateDir string
}

type secretsBackend interface {
	Save(creds Credentials) error
	Load() (Credentials, error)
	Delete() error
}

type store struct {
	insecure  bool
	keyring   *keyringSecrets
	file      *plainFileSecrets
	statePath string
	lockPath  string
}

func New(opts Options) (Store, error) {
	if opts.Context == "" {
		return nil, errors.New("credstore: context is required")
	}
	stateDir, err := resolveStateDir(opts.StateDir)
	if err != nil {
		return nil, fmt.Errorf("credstore: resolve state dir: %w", err)
	}
	return &store{
		insecure:  opts.Insecure,
		keyring:   &keyringSecrets{user: "credentials/" + opts.Context},
		file:      &plainFileSecrets{path: filepath.Join(stateDir, "credentials-"+opts.Context+".json")},
		statePath: filepath.Join(stateDir, "state-"+opts.Context+".json"),
		lockPath:  filepath.Join(stateDir, "lock-"+opts.Context),
	}, nil
}

func (s *store) backend(insecure bool) secretsBackend {
	if insecure {
		return s.file
	}
	return s.keyring
}

func resolveStateDir(override string) (string, error) {
	dir := override
	if dir == "" {
		dir = os.Getenv("LFED_STATE_DIR")
	}
	if dir == "" {

		if xdgState := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(xdgState) {
			dir = filepath.Join(xdgState, "lfed")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("determine home directory: %w", err)
			}
			dir = filepath.Join(home, ".local", "state", "lfed")
		}
	}
	if err := MkdirPrivate(dir); err != nil {
		return "", fmt.Errorf("create state directory %q: %w", dir, err)
	}
	return dir, nil
}

func MkdirPrivate(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil { // #nosec G703
		return err
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	info, err := os.Stat(dir) // #nosec G703
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0o077 != 0 {
		return os.Chmod(dir, 0o700) // #nosec G302,G703
	}
	return nil
}

func WriteOwnerOnlyFile(path string, data []byte) (err error) {
	if runtime.GOOS != "windows" {
		if info, statErr := os.Lstat(path); statErr == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o077 != 0 {
			if err := os.Chmod(path, 0o600); err != nil {
				return err
			}
		}
	}

	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *store) SaveCredentials(creds Credentials) error { return s.backend(s.insecure).Save(creds) }

func (s *store) LoadCredentials() (Credentials, error) { return s.backend(s.insecure).Load() }

func (s *store) DeleteCredentials(insecure bool) error { return s.backend(insecure).Delete() }

func (s *store) Lock(ctx context.Context) (func(), error) {
	waitCtx, cancel := context.WithTimeout(ctx, LockTimeout)
	defer cancel()
	lock := flock.New(s.lockPath)
	locked, err := lock.TryLockContext(waitCtx, lockRetry)
	switch {
	case err != nil && ctx.Err() != nil:
		return nil, fmt.Errorf("credstore: lock %s: %w", s.lockPath, ctx.Err())
	case errors.Is(err, context.DeadlineExceeded):
		return nil, fmt.Errorf("credstore: another lfed process holds %s; waited %s", s.lockPath, LockTimeout)
	case err != nil:
		return nil, fmt.Errorf("credstore: lock %s: %w", s.lockPath, err)
	case !locked:
		return nil, fmt.Errorf("credstore: lock %s: not acquired", s.lockPath)
	}
	return func() { _ = lock.Unlock() }, nil
}

func (s *store) SaveDeviceState(state DeviceState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("credstore: marshal device state: %w", err)
	}
	if err := WriteOwnerOnlyFile(s.statePath, data); err != nil {
		return fmt.Errorf("credstore: write device state: %w", err)
	}
	return nil
}

func (s *store) LoadDeviceState() (DeviceState, error) {
	data, err := os.ReadFile(s.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return DeviceState{}, ErrNotFound
	}
	if err != nil {
		return DeviceState{}, fmt.Errorf("credstore: read device state: %w", err)
	}
	var state DeviceState
	if err := json.Unmarshal(data, &state); err != nil {
		return DeviceState{}, fmt.Errorf("credstore: unmarshal device state: %w", err)
	}
	return state, nil
}

func (s *store) DeleteDeviceState() error {
	err := os.Remove(s.statePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("credstore: delete device state: %w", err)
	}
	return nil
}

type keyringSecrets struct {
	user string
}

func (k *keyringSecrets) Save(creds Credentials) error {
	data, err := json.Marshal(creds) // #nosec G117
	if err != nil {
		return fmt.Errorf("credstore: marshal credentials: %w", err)
	}
	if err := keyring.Set(serviceName, k.user, string(data)); err != nil {
		return fmt.Errorf("credstore: save credentials (use --insecure-storage as a fallback): %w", err)
	}
	return nil
}

func (k *keyringSecrets) Load() (Credentials, error) {
	data, err := keyring.Get(serviceName, k.user)
	if errors.Is(err, keyring.ErrNotFound) {
		return Credentials{}, ErrNotFound
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("credstore: load credentials (use --insecure-storage as a fallback): %w", err)
	}
	var creds Credentials
	if err := json.Unmarshal([]byte(data), &creds); err != nil {
		return Credentials{}, fmt.Errorf("credstore: unmarshal credentials: %w", err)
	}
	return creds, nil
}

func (k *keyringSecrets) Delete() error {
	err := keyring.Delete(serviceName, k.user)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("credstore: delete credentials: %w", err)
	}
	return nil
}

type plainFileSecrets struct {
	path string
}

func (p *plainFileSecrets) Save(creds Credentials) error {
	data, err := json.MarshalIndent(creds, "", "  ") // #nosec G117
	if err != nil {
		return fmt.Errorf("credstore: marshal credentials: %w", err)
	}
	if err := WriteOwnerOnlyFile(p.path, data); err != nil {
		return fmt.Errorf("credstore: write credentials: %w", err)
	}
	return nil
}

func (p *plainFileSecrets) Load() (Credentials, error) {
	data, err := os.ReadFile(p.path)
	if errors.Is(err, os.ErrNotExist) {
		return Credentials{}, ErrNotFound
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("credstore: read credentials: %w", err)
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return Credentials{}, fmt.Errorf("credstore: unmarshal credentials: %w", err)
	}
	return creds, nil
}

func (p *plainFileSecrets) Delete() error {
	err := os.Remove(p.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("credstore: delete credentials: %w", err)
	}
	return nil
}
