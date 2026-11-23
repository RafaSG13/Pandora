package store

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// auditMaxBytes is the size at which audit.log is rotated to audit.log.1; the
// previous audit.log.1 is discarded, so the log never holds much more than
// twice this. A variable so tests can shrink it.
var auditMaxBytes int64 = 1 << 20

const aadAudit = "pandora-audit"

// AuditEntry is one recorded action. Title is empty for actions that are not
// about one secret (a metadata-only export).
type AuditEntry struct {
	Time   time.Time `json:"time"`
	Action string    `json:"action"`
	Title  string    `json:"title"`
}

var auditOn atomic.Bool

// SetAuditEnabled turns RecordAudit on or off. Callers set it once from the
// audit_log setting; it is off by default.
func SetAuditEnabled(on bool) { auditOn.Store(on) }

func auditPath() (string, error) {
	p, err := vaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "audit.log"), nil
}

// RecordAudit appends one entry to the audit log when auditing is enabled,
// and does nothing otherwise. It is the only writer of the log, so the TUI and
// the CLI cannot diverge. Each line is the entry sealed under the vault's data
// key (AAD "pandora-audit", fresh nonce, base64), so no title or time is
// readable on disk, and it never carries a value. The vault must be unlocked.
// It takes the vault write lock, so callers must not hold it.
func RecordAudit(action, title string) error {
	if !auditOn.Load() {
		return nil
	}
	key, err := currentKey()
	if err != nil {
		return err
	}
	defer zero(key)
	plain, err := json.Marshal(AuditEntry{Time: time.Now().UTC(), Action: action, Title: title})
	if err != nil {
		return err
	}
	sealed, err := seal(key, plain, []byte(aadAudit))
	if err != nil {
		return err
	}
	line := append([]byte(base64.StdEncoding.EncodeToString(sealed)), '\n')

	path, err := auditPath()
	if err != nil {
		return err
	}
	release, err := lockVault()
	if err != nil {
		return err
	}
	defer release()
	if info, err := os.Stat(path); err == nil && info.Size()+int64(len(line)) > auditMaxBytes {
		if err := os.Remove(path + ".1"); err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.Rename(path, path+".1"); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(line); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// ReadAudit returns the entries recorded at or after since (all of them when
// since is zero), oldest first, and how many lines could not be read: damaged,
// or sealed under another key. The vault must be unlocked. A missing log is an
// empty one.
func ReadAudit(since time.Time) (entries []AuditEntry, unreadable int, err error) {
	key, err := currentKey()
	if err != nil {
		return nil, 0, err
	}
	defer zero(key)
	path, err := auditPath()
	if err != nil {
		return nil, 0, err
	}
	for _, p := range []string{path + ".1", path} {
		got, bad, err := readAuditFile(p, key)
		if err != nil {
			return nil, 0, err
		}
		unreadable += bad
		for _, e := range got {
			if !e.Time.Before(since) {
				entries = append(entries, e)
			}
		}
	}
	return entries, unreadable, nil
}

func readAuditFile(path string, key []byte) (entries []AuditEntry, unreadable int, err error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	} else if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		raw, err := base64.StdEncoding.DecodeString(sc.Text())
		if err != nil {
			unreadable++
			continue
		}
		plain, err := open(key, raw, []byte(aadAudit))
		if err != nil {
			unreadable++
			continue
		}
		var e AuditEntry
		if json.Unmarshal(plain, &e) != nil {
			unreadable++
			continue
		}
		entries = append(entries, e)
	}
	if err := sc.Err(); err != nil {
		unreadable++ // an over-long or cut-off line
	}
	return entries, unreadable, nil
}

// AuditExists reports whether an audit log has been written.
func AuditExists() bool {
	p, err := auditPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}
