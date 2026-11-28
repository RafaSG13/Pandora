package store

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
)

const (
	aadSharing     = "pandora-v2-sharing"
	shareKeyPrefix = "pandora-share-1:"
	shareFormat    = "pandora-share"
	shareVersion   = 1
	shareSigDomain = "pandora-share-v1"
	armorHeader    = "-----BEGIN AGE ENCRYPTED FILE-----"
)

// ShareKeyPrefix starts every share key printed by `pandora id`.
const ShareKeyPrefix = shareKeyPrefix

var (
	ErrInvalidShareKey = errors.New("not a Pandora share key (expected the output of 'pandora id')")
	ErrOwnShareKey     = errors.New("that is this vault's own share key")
	ErrInvalidPeerName = errors.New("peer names are 1-32 characters: letters, digits, '.', '_' or '-'")
	ErrPeerExists      = errors.New("a peer with that name already exists")
	ErrPeerNotFound    = errors.New("no such peer")
	ErrNotShare        = errors.New("not a Pandora share file")
	ErrNotForThisVault = errors.New("this share file was not made for this vault (compare 'pandora id' here with the sender's --to)")
	ErrUnknownSender   = errors.New("the share file was signed by a vault that is not a trusted peer")
	ErrBadSignature    = errors.New("the share file's signature does not verify: it was altered, or not made by the peer it names")
)

var peerNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)

// Peer is another vault trusted to exchange shared secrets with this one.
type Peer struct {
	Name string `json:"name"`
	Key  string `json:"key"` // its share key, as printed by `pandora id`
}

type sharing struct {
	Identity string `json:"identity"`
	SignSeed []byte `json:"sign_seed"`
	Peers    []Peer `json:"peers,omitempty"`
}

func corruptSharing(detail string) error {
	return fmt.Errorf("%w: %s", ErrVaultCorrupt, detail)
}

func (vf vaultFile) sharing(key []byte) (*sharing, error) {
	if len(vf.Sharing) == 0 {
		return nil, nil
	}
	plain, err := open(key, vf.Sharing, []byte(aadSharing))
	if err != nil {
		return nil, corruptSharing("the sharing data was modified or is damaged")
	}
	var s sharing
	if err := json.Unmarshal(plain, &s); err != nil {
		return nil, corruptSharing(err.Error())
	}
	return &s, nil
}

func (vf *vaultFile) setSharing(key []byte, s *sharing) error {
	plain, err := json.Marshal(s)
	if err != nil {
		return err
	}
	vf.Sharing, err = seal(key, plain, []byte(aadSharing))
	return err
}

func newSharing() (*sharing, error) {
	id, err := age.GenerateHybridIdentity()
	if err != nil {
		return nil, err
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &sharing{Identity: id.String(), SignSeed: priv.Seed()}, nil
}

func (s *sharing) keys() (*age.HybridIdentity, ed25519.PrivateKey, error) {
	id, err := age.ParseHybridIdentity(s.Identity)
	if err != nil {
		return nil, nil, corruptSharing("invalid share identity")
	}
	if len(s.SignSeed) != ed25519.SeedSize {
		return nil, nil, corruptSharing("invalid signing key")
	}
	return id, ed25519.NewKeyFromSeed(s.SignSeed), nil
}

func (s *sharing) shareKey() (string, error) {
	id, priv, err := s.keys()
	if err != nil {
		return "", err
	}
	pub := priv.Public().(ed25519.PublicKey)
	return shareKeyPrefix + id.Recipient().String() + ":" + base64.RawURLEncoding.EncodeToString(pub), nil
}

func parseShareKey(k string) (*age.HybridRecipient, ed25519.PublicKey, error) {
	k = strings.TrimSpace(k)
	if !strings.HasPrefix(k, shareKeyPrefix) {
		return nil, nil, ErrInvalidShareKey
	}
	rest := strings.TrimPrefix(k, shareKeyPrefix)
	i := strings.LastIndex(rest, ":")
	if i < 0 {
		return nil, nil, ErrInvalidShareKey
	}
	rcpt, err := age.ParseHybridRecipient(rest[:i])
	if err != nil {
		return nil, nil, ErrInvalidShareKey
	}
	pub, err := base64.RawURLEncoding.DecodeString(rest[i+1:])
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, nil, ErrInvalidShareKey
	}
	return rcpt, ed25519.PublicKey(pub), nil
}

func loadSharing() (*sharing, error) {
	key, err := currentKey()
	if err != nil {
		return nil, err
	}
	defer zero(key)
	vf, err := readVault()
	if err != nil {
		return nil, err
	}
	return vf.sharing(key)
}

// updateSharing applies fn to the sharing state under the write lock,
// creating the identity first if there is none, and writes the vault.
func updateSharing(fn func(*sharing) error) (*sharing, error) {
	key, err := currentKey()
	if err != nil {
		return nil, err
	}
	defer zero(key)
	release, err := lockVault()
	if err != nil {
		return nil, err
	}
	defer release()
	vf, err := readVault()
	if err != nil {
		return nil, err
	}
	if _, err := openRecords(vf, key); err != nil {
		return nil, err
	}
	s, err := vf.sharing(key)
	if err != nil {
		return nil, err
	}
	created := s == nil
	if created {
		if s, err = newSharing(); err != nil {
			return nil, err
		}
	}
	if fn != nil {
		if err := fn(s); err != nil {
			return nil, err
		}
	}
	if err := vf.setSharing(key, s); err != nil {
		return nil, err
	}
	if err := writeVault(vf); err != nil {
		return nil, err
	}
	return s, nil
}

// ensureSharing returns the sharing state, creating the identity if needed.
func ensureSharing() (*sharing, error) {
	s, err := loadSharing()
	if err != nil || s != nil {
		return s, err
	}
	return updateSharing(nil)
}

// ShareKey returns this vault's share key, creating the identity on first use.
func ShareKey() (string, error) {
	s, err := ensureSharing()
	if err != nil {
		return "", err
	}
	return s.shareKey()
}

// Fingerprint is a short, comparable digest of a share key.
func Fingerprint(key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	h := hex.EncodeToString(sum[:16])
	var parts []string
	for i := 0; i < len(h); i += 4 {
		parts = append(parts, h[i:i+4])
	}
	return strings.Join(parts, " ")
}

// Peers lists the trusted peers by name.
func Peers() ([]Peer, error) {
	s, err := loadSharing()
	if err != nil {
		return nil, err
	}
	out := []Peer{}
	if s != nil {
		out = append(out, s.Peers...)
	}
	slices.SortFunc(out, func(a, b Peer) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (s *sharing) checkPeer(name, key string) error {
	if !peerNameRe.MatchString(name) {
		return ErrInvalidPeerName
	}
	if _, _, err := parseShareKey(key); err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if s != nil {
		if own, err := s.shareKey(); err == nil && own == key {
			return ErrOwnShareKey
		}
		for _, p := range s.Peers {
			if p.Name == name {
				return ErrPeerExists
			}
			if p.Key == key {
				return fmt.Errorf("that key is already trusted as peer %q", p.Name)
			}
		}
	}
	return nil
}

// CheckPeer validates a new peer without writing anything.
func CheckPeer(name, key string) error {
	s, err := loadSharing()
	if err != nil {
		return err
	}
	return s.checkPeer(name, key)
}

// AddPeer trusts the vault owning key under name.
func AddPeer(name, key string) error {
	key = strings.TrimSpace(key)
	_, err := updateSharing(func(s *sharing) error {
		if err := s.checkPeer(name, key); err != nil {
			return err
		}
		s.Peers = append(s.Peers, Peer{Name: name, Key: key})
		return nil
	})
	return err
}

// RemovePeer stops trusting a peer.
func RemovePeer(name string) error {
	_, err := updateSharing(func(s *sharing) error {
		i := slices.IndexFunc(s.Peers, func(p Peer) bool { return p.Name == name })
		if i < 0 {
			return fmt.Errorf("%w: %q", ErrPeerNotFound, name)
		}
		s.Peers = slices.Delete(s.Peers, i, i+1)
		return nil
	})
	return err
}

type shareEnvelope struct {
	Format  string   `json:"format"`
	Version int      `json:"version"`
	From    string   `json:"from"`
	To      []string `json:"to"`
	Payload []byte   `json:"payload"`
	Sig     []byte   `json:"sig"`
}

func shareMessage(from string, to []string, payload []byte) []byte {
	var b bytes.Buffer
	b.WriteString(shareSigDomain)
	b.WriteByte(0)
	b.WriteString(from)
	b.WriteByte(0)
	b.WriteString(strings.Join(to, "\n"))
	b.WriteByte(0)
	b.Write(payload)
	return b.Bytes()
}

// SealShare signs payload and encrypts it for the named peers.
func SealShare(payload []byte, peerNames []string) ([]byte, error) {
	if len(peerNames) == 0 {
		return nil, errors.New("no peer given")
	}
	s, err := ensureSharing()
	if err != nil {
		return nil, err
	}
	_, priv, err := s.keys()
	if err != nil {
		return nil, err
	}
	from, err := s.shareKey()
	if err != nil {
		return nil, err
	}
	var to []string
	var rcpts []age.Recipient
	seen := map[string]bool{}
	for _, n := range peerNames {
		i := slices.IndexFunc(s.Peers, func(p Peer) bool { return p.Name == n })
		if i < 0 {
			return nil, fmt.Errorf("%w: %q", ErrPeerNotFound, n)
		}
		k := s.Peers[i].Key
		if seen[k] {
			continue
		}
		seen[k] = true
		r, _, err := parseShareKey(k)
		if err != nil {
			return nil, err
		}
		to = append(to, k)
		rcpts = append(rcpts, r)
	}
	slices.Sort(to)
	env := shareEnvelope{Format: shareFormat, Version: shareVersion, From: from, To: to, Payload: payload}
	env.Sig = ed25519.Sign(priv, shareMessage(from, to, payload))
	plain, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	armored := armor.NewWriter(&buf)
	w, err := age.Encrypt(armored, rcpts...)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(plain); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if err := armored.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// OpenShare decrypts a share file made for this vault and verifies that a
// trusted peer signed it. It writes nothing.
func OpenShare(data []byte) ([]byte, Peer, error) {
	s, err := loadSharing()
	if err != nil {
		return nil, Peer{}, err
	}
	var id *age.HybridIdentity
	var own string
	if s != nil {
		if id, _, err = s.keys(); err != nil {
			return nil, Peer{}, err
		}
		if own, err = s.shareKey(); err != nil {
			return nil, Peer{}, err
		}
	} else if id, err = age.GenerateHybridIdentity(); err != nil {
		return nil, Peer{}, err
	}
	var src io.Reader = bytes.NewReader(data)
	if bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte(armorHeader)) {
		src = armor.NewReader(src)
	}
	r, err := age.Decrypt(src, id)
	if err != nil {
		var noMatch *age.NoIdentityMatchError
		if errors.As(err, &noMatch) {
			if slices.Contains(noMatch.StanzaTypes, "scrypt") {
				return nil, Peer{}, ErrNotShare
			}
			return nil, Peer{}, ErrNotForThisVault
		}
		return nil, Peer{}, fmt.Errorf("cannot decrypt: %w", err)
	}
	plain, err := io.ReadAll(r)
	if err != nil {
		return nil, Peer{}, fmt.Errorf("cannot decrypt: file is corrupt or truncated: %w", err)
	}
	var env shareEnvelope
	if json.Unmarshal(plain, &env) != nil || env.Format != shareFormat || env.Version != shareVersion {
		return nil, Peer{}, ErrNotShare
	}
	var peer *Peer
	if s != nil {
		for i := range s.Peers {
			if s.Peers[i].Key == env.From {
				peer = &s.Peers[i]
			}
		}
	}
	if peer == nil {
		return nil, Peer{}, fmt.Errorf("%w (fingerprint %s); check it on the sending machine with 'pandora id', then 'pandora peer add'", ErrUnknownSender, Fingerprint(env.From))
	}
	_, pub, err := parseShareKey(peer.Key)
	if err != nil || !ed25519.Verify(pub, shareMessage(env.From, env.To, env.Payload), env.Sig) {
		return nil, Peer{}, ErrBadSignature
	}
	if !slices.Contains(env.To, own) {
		return nil, Peer{}, ErrNotForThisVault
	}
	return env.Payload, *peer, nil
}
