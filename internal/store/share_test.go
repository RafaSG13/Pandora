package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"filippo.io/age/armor"
)

// useVault switches to the vault in dir (creating it), keeping the mock keyring.
func useVault(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	t.Setenv("AppData", dir)
	Lock()
	var err error
	if Exists() {
		err = Unlock(testPassword)
	} else {
		err = Create(testPassword)
	}
	if err != nil {
		t.Fatal(err)
	}
}

type machines struct{ a, b, c string }

// pair sets up machines A and B trusting each other (A names B "laptop", B
// names A "desktop"), plus an empty C. It leaves A active.
func pair(t *testing.T) (m machines, keyA, keyB string) {
	t.Helper()
	setup(t) // A (fresh mock keyring)
	m = machines{a: t.TempDir(), b: t.TempDir(), c: t.TempDir()}
	// setup created A in the HOME it set; move A to m.a to have a path for it.
	Lock()
	useVault(t, m.a)
	var err error
	if keyA, err = ShareKey(); err != nil {
		t.Fatal(err)
	}
	useVault(t, m.b)
	if keyB, err = ShareKey(); err != nil {
		t.Fatal(err)
	}
	if err := AddPeer("desktop", keyA); err != nil {
		t.Fatal(err)
	}
	useVault(t, m.a)
	if err := AddPeer("laptop", keyB); err != nil {
		t.Fatal(err)
	}
	return m, keyA, keyB
}

func TestShareKeyIsStable(t *testing.T) {
	setup(t)
	k1, err := ShareKey()
	if err != nil || !strings.HasPrefix(k1, ShareKeyPrefix) {
		t.Fatalf("ShareKey = %q, %v", k1, err)
	}
	mustSave(t, "x", "c", "v")
	newPw := "another long password"
	if err := ChangePassword(testPassword, newPw); err != nil {
		t.Fatal(err)
	}
	Lock()
	if err := Unlock(newPw); err != nil {
		t.Fatal(err)
	}
	if k2, _ := ShareKey(); k2 != k1 {
		t.Errorf("share key changed: %q -> %q", k1, k2)
	}
}

func TestSharingIsEncryptedInVaultFile(t *testing.T) {
	m, _, keyB := pair(t)
	_ = m
	_ = keyB
	raw := string(readVaultBytes(t))
	for _, bad := range []string{"laptop", "AGE-SECRET-KEY", "pandora-share"} {
		if strings.Contains(raw, bad) {
			t.Errorf("vault.json contains %q in clear", bad)
		}
	}
}

func shareFrom(t *testing.T, to ...string) []byte {
	t.Helper()
	out, err := SealShare([]byte(`{"secrets":[]}`), to)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestShareRoundTripAndRejections(t *testing.T) {
	m, _, _ := pair(t)
	file := shareFrom(t, "laptop")
	if strings.Contains(string(file), "secrets") {
		t.Error("share file leaks payload")
	}

	useVault(t, m.b)
	payload, from, err := OpenShare(file)
	if err != nil || from.Name != "desktop" || string(payload) != `{"secrets":[]}` {
		t.Fatalf("OpenShare = %q, %+v, %v", payload, from, err)
	}

	// A machine that does not hold the recipient identity.
	useVault(t, m.c)
	if _, _, err := OpenShare(file); !errors.Is(err, ErrNotForThisVault) {
		t.Errorf("C: err = %v, want ErrNotForThisVault", err)
	}

	// B drops A: unknown sender.
	useVault(t, m.b)
	if err := RemovePeer("desktop"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := OpenShare(file); !errors.Is(err, ErrUnknownSender) {
		t.Errorf("err = %v, want ErrUnknownSender", err)
	}

	// scrypt file is not a share, also in a vault with no identity.
	rc, _ := age.NewScryptRecipient("some passphrase")
	rc.SetWorkFactor(10)
	var buf bytes.Buffer
	w, _ := age.Encrypt(&buf, rc)
	w.Write([]byte("x"))
	w.Close()
	for _, dir := range []string{m.b, m.c} {
		useVault(t, dir)
		if _, _, err := OpenShare(buf.Bytes()); !errors.Is(err, ErrNotShare) {
			t.Errorf("scrypt: err = %v, want ErrNotShare", err)
		}
	}
}

// reseal decrypts a share addressed to the vault in from, lets edit change the
// envelope, and encrypts it again for the keys in to.
func reseal(t *testing.T, file []byte, from string, to []string, edit func(*shareEnvelope)) []byte {
	t.Helper()
	useVault(t, from)
	s, err := loadSharing()
	if err != nil {
		t.Fatal(err)
	}
	id, _, _ := s.keys()
	r, err := age.Decrypt(armor.NewReader(bytes.NewReader(file)), id)
	if err != nil {
		t.Fatal(err)
	}
	var env shareEnvelope
	buf := new(bytes.Buffer)
	buf.ReadFrom(r)
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(&env)
	}
	plain, _ := json.Marshal(env)
	var rcpts []age.Recipient
	for _, k := range to {
		rc, _, err := parseShareKey(k)
		if err != nil {
			t.Fatal(err)
		}
		rcpts = append(rcpts, rc)
	}
	var out bytes.Buffer
	aw := armor.NewWriter(&out)
	w, _ := age.Encrypt(aw, rcpts...)
	w.Write(plain)
	w.Close()
	aw.Close()
	return out.Bytes()
}

func TestShareTamperedAndForwarded(t *testing.T) {
	m, _, keyB := pair(t)
	file := shareFrom(t, "laptop")

	tampered := reseal(t, file, m.b, []string{keyB}, func(e *shareEnvelope) { e.Payload = []byte(`{"secrets":[1]}`) })
	useVault(t, m.b)
	if _, _, err := OpenShare(tampered); !errors.Is(err, ErrBadSignature) {
		t.Errorf("tampered: err = %v, want ErrBadSignature", err)
	}

	// B forwards A's envelope untouched to C, who trusts A.
	useVault(t, m.c)
	keyC, _ := ShareKey()
	keyA := func() string { useVault(t, m.a); k, _ := ShareKey(); return k }()
	useVault(t, m.c)
	if err := AddPeer("desktop", keyA); err != nil {
		t.Fatal(err)
	}
	fwd := reseal(t, file, m.b, []string{keyC}, nil)
	useVault(t, m.c)
	if _, _, err := OpenShare(fwd); !errors.Is(err, ErrNotForThisVault) {
		t.Errorf("forwarded: err = %v, want ErrNotForThisVault", err)
	}
}

func TestCheckPeerRejections(t *testing.T) {
	_, keyA, keyB := pair(t) // A active
	own, _ := ShareKey()
	if own != keyA {
		t.Fatal("A's key mismatch")
	}
	cases := []struct {
		name, key string
		want      error
	}{
		{"x", own, ErrOwnShareKey},
		{"laptop", keyB, ErrPeerExists},
		{"bad name!", keyB, ErrInvalidPeerName},
		{"other", "pandora-share-1:nope", ErrInvalidShareKey},
		{"other", "garbage", ErrInvalidShareKey},
	}
	for _, c := range cases {
		if err := CheckPeer(c.name, c.key); !errors.Is(err, c.want) {
			t.Errorf("CheckPeer(%q): %v, want %v", c.name, err, c.want)
		}
	}
	if err := CheckPeer("again", keyB); err == nil || !strings.Contains(err.Error(), "already trusted") {
		t.Errorf("same key, other name: %v", err)
	}
}
