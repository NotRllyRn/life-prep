package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	base := `{"Database":"db","Artifacts":"artifacts"`
	for _, tail := range []string{`,"Typo":true}`, `,"Hermes":{"Enabld":true}}`, `,"Accounts":[{"ID":"a","Typo":"x"}]}`, `,"Accounts":[{"ID":"a"},{"ID":"a"}]}`, `,"Accounts":[{"ID":"a","Email":"USER@example.com"},{"ID":"b","Email":"user@example.com"}]}`, `,"Hermes":{"URL":"http://example.com/hook"}}`, `,"Hermes":{"URL":"https://user:pass@example.com/hook"}}`, `,"Hermes":{"Enabled":true}}`, `,"Enabled":true}`} {
		if _, err := decodeConfig([]byte(base + tail)); err == nil {
			t.Errorf("accepted invalid config: %s", tail)
		}
	}
	for _, u := range []string{"https://example.com/hook", "http://127.0.0.1:8644/hook", "http://[::1]:8644/hook", "http://localhost/hook"} {
		if _, err := decodeConfig([]byte(base + `,"Hermes":{"URL":"` + u + `"}}`)); err != nil {
			t.Fatal(u, err)
		}
	}
	if _, err := decodeConfig([]byte(base + `} {}`)); err == nil {
		t.Fatal("trailing JSON accepted")
	}
}

func TestDisabledServeAndDoctor(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")
	if err := os.WriteFile(p, []byte(`{"Database":"`+filepath.Join(dir, "state.db")+`","Artifacts":"`+filepath.Join(dir, "artifacts")+`"}`), 0600); err != nil {
		t.Fatal(err)
	}
	old := os.Args
	defer func() { os.Args = old }()
	for _, cmd := range []string{"serve", "doctor", "backfill", "watch"} {
		os.Args = []string{"life-prep", cmd, p}
		if err := run(); err == nil {
			t.Fatal("disabled command succeeded", cmd)
		}
	}
}
