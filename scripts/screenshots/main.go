//go:build ignore

// Command pandora-demo runs the Pandora interface on a throwaway vault and an
// in-memory keyring, seeded with fake secrets. scripts/screenshots/record.sh
// uses it to regenerate the README images; it never touches your real vault or
// OS keyring.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zalando/go-keyring"

	"github.com/RafaSG13/Pandora/internal/config"
	"github.com/RafaSG13/Pandora/internal/store"
	"github.com/RafaSG13/Pandora/internal/tui"
)

type demoSecret struct {
	title, category, username, notes, value string
	tags                                    []string
	createdAgo                              int
	expiresIn                               *int
}

var secrets = []demoSecret{
	{"github-token", "Tokens", "rafa", "https://github.com/settings/tokens", "demo-github-7Fq2Lw9Xk3Rt", []string{"ci", "prod"}, 40, new(60)},
	{"stripe-api-key", "Payments", "", "Dashboard > Developers > API keys", "demo-stripe-Hn4Vb8Qz1Mc6", []string{"prod"}, 30, new(7)},
	{"aws-prod-access", "Cloud", "deploy-bot", "", "demo-aws-Kd7Pw2Ys5Lr9Tx", []string{"prod", "aws"}, 90, new(-3)},
	{"postgres-staging", "Databases", "app_rw", "postgres://db.staging.internal:5432/app", "demo-pg-Wm3Jc8Nv1Qa4", []string{"staging", "db"}, 20, nil},
	{"openai-api-key", "Tokens", "", "", "demo-openai-Zr6Tb2Hx9Ue5", []string{"ai"}, 12, nil},
	{"ssh-deploy-key", "Infra", "deploy", "", "-----BEGIN DEMO KEY-----\nbm90IGEgcmVhbCBrZXkgLSBkZW1vIG9ubHk=\n-----END DEMO KEY-----", []string{"ssh"}, 8, nil},
	{"slack-webhook", "Notifications", "", "#deploys channel", "demo-slack-Pq5Ld1Gs7Vy3", []string{"alerts"}, 5, nil},
	{"wifi-office", "Personal", "guest", "", "demo-wifi-Bt8Kx4Rm2Nz6", nil, 2, nil},
}

func setup(dir string) error {
	os.Setenv("HOME", dir)
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, ".config"))
	os.Setenv("AppData", dir)
	keyring.MockInit()

	p, err := config.Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(p, []byte(`{"background": "dark"}`), 0o600); err != nil {
		return err
	}
	if err := store.Create("demo-master-password"); err != nil {
		return err
	}
	now := time.Now()
	for _, s := range secrets {
		meta := store.SecretMeta{
			Title:    s.title,
			Category: s.category,
			Username: s.username,
			Notes:    s.notes,
			Tags:     s.tags,
			Created:  now.AddDate(0, 0, -s.createdAgo).Format(store.DateLayout),
		}
		if s.expiresIn != nil {
			meta.Expires = now.AddDate(0, 0, *s.expiresIn).Format(store.DateLayout)
		}
		if err := store.SaveSecret(meta, s.value); err != nil {
			return fmt.Errorf("seed %s: %w", s.title, err)
		}
	}
	return nil
}

func main() {
	dir, err := os.MkdirTemp("", "pandora-demo-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "pandora-demo: %v\n", err)
		os.Exit(1)
	}
	if err := setup(dir); err != nil {
		fmt.Fprintf(os.Stderr, "pandora-demo: %v\n", err)
		os.RemoveAll(dir)
		os.Exit(1)
	}
	_, err = tea.NewProgram(tui.New(), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	os.RemoveAll(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pandora-demo: %v\n", err)
		os.Exit(1)
	}
}
