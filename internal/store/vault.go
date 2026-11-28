package store

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

// MinPasswordLen is the minimum length, in runes, of the master password.
// There are no composition rules.
const MinPasswordLen = 10

var (
	// ErrLocked is returned by every store function that needs the vault while
	// no master password has been entered (or after Lock).
	ErrLocked = errors.New("vault is locked")
	// ErrWrongPassword is returned when the master password does not unlock the vault.
	ErrWrongPassword = errors.New("wrong master password")
	// ErrPasswordTooShort is returned when a new master password is shorter than MinPasswordLen.
	ErrPasswordTooShort = fmt.Errorf("master password must be at least %d characters", MinPasswordLen)
	// ErrInvalidText is returned for a title, category or note containing control or invisible formatting characters.
	ErrInvalidText = errors.New("text must not contain control characters or invisible formatting characters")
	// ErrVaultExists is returned by Create when a vault is already present.
	ErrVaultExists = errors.New("a vault already exists")
	// ErrKeyChanged is returned when another process replaced the data key since
	// this one unlocked; the session is locked and the master password is needed again.
	ErrKeyChanged = errors.New("the vault key was changed by another Pandora process; unlock again")
	// ErrVaultCorrupt is returned (wrapped) when vault.json cannot be parsed,
	// has invalid parameters, or its data does not decrypt or parse. Callers
	// never overwrite such a file; `pandora doctor --fix` restores the backup.
	ErrVaultCorrupt = errors.New("vault file is corrupt")
	// errNewerVault marks a vault written by a newer Pandora: never "repaired".
	errNewerVault = errors.New("unsupported vault version")
)

const (
	vaultVersion = 2
	vaultKDF     = "argon2id"
	aadKey       = "pandora-v2-key"
	aadMeta      = "pandora-v2-meta"
	valuePrefix  = "pv2:"
	keyLen       = chacha20poly1305.KeySize
	saltLen      = 16
)

// kdfParams are the Argon2id settings stored in vault.json. They are read from
// the file, which is untrusted input, so checkKDF bounds them.
type kdfParams struct {
	Alg       string `json:"alg"`
	Salt      []byte `json:"salt"`
	Time      uint32 `json:"time"`
	MemoryKiB uint32 `json:"memory_kib"`
	Threads   uint8  `json:"threads"`
}

// defaultKDF holds the parameters used for new vaults and password changes.
var defaultKDF = kdfParams{Alg: vaultKDF, Time: 3, MemoryKiB: 64 * 1024, Threads: 4}

// FastKDFForTests makes new vaults use trivially cheap Argon2 parameters.
// Tests only: never call it from production code.
func FastKDFForTests() { defaultKDF = kdfParams{Alg: vaultKDF, Time: 1, MemoryKiB: 8, Threads: 1} }

// vaultFile is the on-disk form of vault.json. WrappedKey and Data are
// nonce(24) || ciphertext.
type vaultFile struct {
	Version    int       `json:"version"`
	KDF        kdfParams `json:"kdf"`
	WrappedKey []byte    `json:"wrapped_key"`
	Data       []byte    `json:"data"`
	// Sharing is the encrypted share identity and trusted peers (share.go);
	// absent until first needed.
	Sharing []byte `json:"sharing,omitempty"`
	// Pending lists keyring entries a key rotation still has to remove
	// (pending.go), sealed under the data key; absent when there are none.
	Pending []byte `json:"pending,omitempty"`
}

// record is one secret in the encrypted metadata: SecretMeta plus the opaque
// ID that is the keyring account of its value.
type record struct {
	ID string `json:"id"`
	// Chunks is how many keyring entries hold the value; absent means one.
	Chunks int `json:"chunks,omitempty"`
	// History holds the previous values, newest first (see history.go).
	History []version `json:"history,omitempty"`
	SecretMeta
}

var session struct {
	mu  sync.Mutex
	key []byte
	// wrapped is the wrapped_key of the vault file the session was unlocked
	// from; a different one later means another process rotated the data key.
	wrapped []byte
}

func vaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "pandora", "vault.json"), nil
}

// Exists reports whether a vault file is present.
func Exists() bool {
	p, err := vaultPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// IsUnlocked reports whether the master password has been entered.
func IsUnlocked() bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.key != nil
}

// Lock forgets the data key. The slice is zeroed, but Go gives no guarantee
// that copies made by the runtime or the OS (swap, core dumps) are wiped.
func Lock() {
	session.mu.Lock()
	defer session.mu.Unlock()
	zero(session.key)
	session.key = nil
	session.wrapped = nil
}

// setSession unlocks with key, the data key wrapped on disk as wrapped.
func setSession(key, wrapped []byte) {
	session.mu.Lock()
	defer session.mu.Unlock()
	zero(session.key)
	session.key = append([]byte(nil), key...)
	session.wrapped = append([]byte(nil), wrapped...)
}

// keyChanged reports whether wrapped differs from the wrapped_key this
// session was unlocked with.
func keyChanged(wrapped []byte) bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	return session.wrapped != nil && !bytes.Equal(wrapped, session.wrapped)
}

// currentKey returns a private copy of the data key; callers zero it after use.
func currentKey() ([]byte, error) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.key == nil {
		return nil, ErrLocked
	}
	return append([]byte(nil), session.key...), nil
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func checkPassword(pw string) error {
	if utf8.RuneCountInString(pw) < MinPasswordLen {
		return ErrPasswordTooShort
	}
	return nil
}

func checkKDF(p kdfParams) error {
	switch {
	case p.Alg != vaultKDF:
		return fmt.Errorf("vault uses unsupported KDF %q", p.Alg)
	case p.Time < 1 || p.Time > 10,
		p.MemoryKiB < 8 || p.MemoryKiB > 1<<20,
		p.Threads < 1 || p.Threads > 16:
		return errors.New("vault has out-of-range KDF parameters")
	case len(p.Salt) != saltLen:
		return errors.New("vault has an invalid KDF salt")
	}
	return nil
}

func deriveKEK(password string, p kdfParams) []byte {
	return argon2.IDKey([]byte(password), p.Salt, p.Time, p.MemoryKiB, p.Threads, keyLen)
}

func seal(key, plaintext, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+len(plaintext)+aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return aead.Seal(nonce, nonce, plaintext, aad), nil
}

func open(key, blob, aad []byte) ([]byte, error) {
	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, err
	}
	if len(blob) < aead.NonceSize()+aead.Overhead() {
		return nil, errors.New("ciphertext too short")
	}
	return aead.Open(nil, blob[:aead.NonceSize()], blob[aead.NonceSize():], aad)
}

// newKDF returns fresh default parameters with a random salt.
func newKDF() (kdfParams, error) {
	p := defaultKDF
	p.Salt = make([]byte, saltLen)
	if _, err := rand.Read(p.Salt); err != nil {
		return kdfParams{}, err
	}
	return p, nil
}

// newVault builds an empty vault protected by password and returns it with its
// data key. Nothing is written.
func newVault(password string) (vaultFile, []byte, error) {
	if err := checkPassword(password); err != nil {
		return vaultFile{}, nil, err
	}
	p, err := newKDF()
	if err != nil {
		return vaultFile{}, nil, err
	}
	key := make([]byte, keyLen)
	if _, err := rand.Read(key); err != nil {
		return vaultFile{}, nil, err
	}
	kek := deriveKEK(password, p)
	defer zero(kek)
	wrapped, err := seal(kek, key, []byte(aadKey))
	if err != nil {
		return vaultFile{}, nil, err
	}
	vf := vaultFile{Version: vaultVersion, KDF: p, WrappedKey: wrapped}
	if err := vf.setRecords(key, []record{}); err != nil {
		return vaultFile{}, nil, err
	}
	return vf, key, nil
}

func (vf *vaultFile) setRecords(key []byte, recs []record) error {
	plain, err := json.Marshal(recs)
	if err != nil {
		return err
	}
	vf.Data, err = seal(key, plain, []byte(aadMeta))
	return err
}

func readVault() (vaultFile, error) {
	p, err := vaultPath()
	if err != nil {
		return vaultFile{}, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return vaultFile{}, err
	}
	return parseVault(raw)
}

// parseVault decodes and bounds-checks the content of a vault file. Anything
// that is not a vault of a version this build reads wraps ErrVaultCorrupt;
// a version from the future wraps errNewerVault instead.
func parseVault(raw []byte) (vaultFile, error) {
	var vf vaultFile
	if err := json.Unmarshal(raw, &vf); err != nil {
		return vf, fmt.Errorf("%w: %v", ErrVaultCorrupt, err)
	}
	switch {
	case vf.Version > vaultVersion:
		return vf, fmt.Errorf("%w %d", errNewerVault, vf.Version)
	case vf.Version != vaultVersion:
		return vf, fmt.Errorf("%w: unsupported vault version %d", ErrVaultCorrupt, vf.Version)
	}
	if err := checkKDF(vf.KDF); err != nil {
		return vf, fmt.Errorf("%w: %v", ErrVaultCorrupt, err)
	}
	return vf, nil
}

// records decrypts and decodes the secret records with the data key.
func (vf vaultFile) records(key []byte) ([]record, error) {
	plain, err := open(key, vf.Data, []byte(aadMeta))
	if err != nil {
		return nil, fmt.Errorf("%w: the data was modified or is damaged", ErrVaultCorrupt)
	}
	var recs []record
	if err := json.Unmarshal(plain, &recs); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVaultCorrupt, err)
	}
	if recs == nil {
		recs = []record{}
	}
	return recs, nil
}

// marshalVault is the byte form of vf written to vault.json.
func marshalVault(vf vaultFile) ([]byte, error) {
	return json.MarshalIndent(vf, "", "  ")
}

func writeVault(vf vaultFile) error {
	p, err := vaultPath()
	if err != nil {
		return err
	}
	out, err := marshalVault(vf)
	if err != nil {
		return err
	}
	return writeFileAtomic(p, out)
}

// commitVault writes vf as vault.json and then as vault.json.bak, so the
// backup never keeps a previous wrapping or data key. err: nothing changed.
// backupErr: vault.json is committed but vault.json.bak still holds the
// previous file (or vault.json.corrupt could not be removed).
func commitVault(vf vaultFile) (backupErr, err error) {
	p, err := vaultPath()
	if err != nil {
		return nil, err
	}
	out, err := marshalVault(vf)
	if err != nil {
		return nil, err
	}
	if err := replaceFile(p, out, false); err != nil {
		return nil, err
	}
	backupErr = replaceFile(p+".bak", out, false)
	// A leftover vault.json.corrupt may hold the previous wrapping.
	if err := os.Remove(p + ".corrupt"); err != nil && !os.IsNotExist(err) {
		backupErr = errors.Join(backupErr, err)
	}
	return backupErr, nil
}

// unwrap derives the key-encryption key from password and opens the data key.
func (vf vaultFile) unwrap(password string) ([]byte, error) {
	kek := deriveKEK(password, vf.KDF)
	defer zero(kek)
	key, err := open(kek, vf.WrappedKey, []byte(aadKey))
	if err != nil {
		return nil, ErrWrongPassword
	}
	return key, nil
}

// Create makes a new empty vault protected by password and leaves it unlocked.
func Create(password string) error {
	release, err := lockVault()
	if err != nil {
		return err
	}
	defer release()
	if Exists() {
		return ErrVaultExists
	}
	vf, key, err := newVault(password)
	if err != nil {
		return err
	}
	defer zero(key)
	if err := writeVault(vf); err != nil {
		return err
	}
	setSession(key, vf.WrappedKey)
	return nil
}

// Unlock opens the vault with password. A wrong password yields ErrWrongPassword.
func Unlock(password string) error {
	vf, err := readVault()
	if err != nil {
		return err
	}
	key, err := vf.unwrap(password)
	if err != nil {
		return err
	}
	defer zero(key)
	setSession(key, vf.WrappedKey)
	return nil
}

// ChangePassword re-wraps the data key under newPw after verifying oldPw. The
// data key itself is unchanged, so values already in the keyring stay valid;
// the previous backup is replaced so it no longer holds the old wrapping.
func ChangePassword(oldPw, newPw string) error {
	release, err := lockVault()
	if err != nil {
		return err
	}
	defer release()
	vf, err := readVault()
	if err != nil {
		return err
	}
	key, err := vf.unwrap(oldPw)
	if err != nil {
		return err
	}
	defer zero(key)
	// Never re-save a vault whose contents do not open: the write would copy
	// the damaged file over the good backup.
	if _, err := vf.records(key); err != nil {
		return err
	}
	if _, err := vf.sharing(key); err != nil {
		return err
	}
	if err := checkPassword(newPw); err != nil {
		return err
	}
	p, err := newKDF()
	if err != nil {
		return err
	}
	kek := deriveKEK(newPw, p)
	defer zero(kek)
	wrapped, err := seal(kek, key, []byte(aadKey))
	if err != nil {
		return err
	}
	vf.KDF, vf.WrappedKey = p, wrapped
	backupErr, err := commitVault(vf)
	if err != nil {
		return err
	}
	if backupErr != nil {
		return &PasswordCleanupError{Problems: []string{"vault.json.bak still holds the old password: " + backupErr.Error()}}
	}
	return nil
}

// loadRecords decrypts the vault records. A missing vault yields no records;
// a corrupt one is an error and is never overwritten by the callers.
func loadRecords() ([]record, error) {
	key, err := currentKey()
	if err != nil {
		return nil, err
	}
	defer zero(key)
	vf, err := readVault()
	if os.IsNotExist(err) {
		return []record{}, nil
	} else if err != nil {
		return nil, err
	}
	return openRecords(vf, key)
}

// openRecords decrypts vf's records with the session key. When they do not
// open and vf's wrapped key differs from the session's, another process
// rotated the data key: the session is locked and ErrKeyChanged returned.
func openRecords(vf vaultFile, key []byte) ([]record, error) {
	recs, err := vf.records(key)
	if err != nil && errors.Is(err, ErrVaultCorrupt) && keyChanged(vf.WrappedKey) {
		Lock()
		return nil, ErrKeyChanged
	}
	return recs, err
}

// saveRecords re-encrypts recs (fresh nonce) into the existing vault file.
func saveRecords(recs []record) error {
	key, err := currentKey()
	if err != nil {
		return err
	}
	defer zero(key)
	vf, err := readVault()
	if err != nil {
		return err
	}
	if err := vf.setRecords(key, recs); err != nil {
		return err
	}
	return writeVault(vf)
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func recordIndex(recs []record, title string) int {
	for i, r := range recs {
		if r.Title == title {
			return i
		}
	}
	return -1
}

// ValidateText rejects control characters (ESC, newline, tab, ...), which
// could inject terminal escape sequences into list output and the TUI, and
// invisible formatting characters (bidirectional overrides, zero-width
// space, BOM) that could make one title look like another. The zero-width
// joiner and non-joiner stay allowed: emoji and some scripts need them.
func ValidateText(s string) error {
	for _, r := range s {
		if unicode.IsControl(r) || invisibleFormat(r) {
			return ErrInvalidText
		}
	}
	return nil
}

// invisibleFormat reports the zero-width and bidirectional formatting
// characters ValidateText rejects.
func invisibleFormat(r rune) bool {
	switch {
	case r == 0x061C, r == 0x200B, r == 0x200E, r == 0x200F,
		r >= 0x202A && r <= 0x202E, r == 0x2060,
		r >= 0x2066 && r <= 0x2069, r == 0xFEFF:
		return true
	}
	return false
}

// writeFileAtomic replaces path with data, first keeping the previous
// contents as path+".bak".
func writeFileAtomic(path string, data []byte) error { return replaceFile(path, data, true) }

// replaceFile replaces path with data through a temp file and rename. The
// temp file and then the directory are fsynced, so a crash leaves either the
// old or the new file, never an empty one. keepBackup first saves the previous
// contents as path+".bak".
func replaceFile(path string, data []byte, keepBackup bool) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if keepBackup {
		if old, err := os.ReadFile(path); err == nil {
			if err := replaceFile(path+".bak", old, false); err != nil {
				return err
			}
		}
	}
	if err := os.Rename(tmp.Name(), path); err != nil { // CreateTemp files are 0600
		return err
	}
	syncDir(dir)
	return nil
}

// syncDir flushes a directory entry change to disk. Best effort: Windows
// cannot fsync a directory handle, and a failure here does not undo the rename.
func syncDir(dir string) {
	d, err := os.Open(dir)
	if err != nil {
		return
	}
	_ = d.Sync()
	_ = d.Close()
}
