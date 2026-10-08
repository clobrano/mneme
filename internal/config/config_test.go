package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.NotesDir == "" {
		t.Error("DefaultConfig.NotesDir should not be empty")
	}
	if cfg.Review.Match != "any" {
		t.Errorf("DefaultConfig.Review.Match = %q, want %q", cfg.Review.Match, "any")
	}
	if len(cfg.Review.Tags) != 0 {
		t.Errorf("DefaultConfig.Review.Tags should be empty, got %v", cfg.Review.Tags)
	}
}

func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load missing file should return no error, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load missing file should return DefaultConfig, got nil")
	}
	if cfg.NotesDir == "" {
		t.Error("Load missing file should return defaults with NotesDir set")
	}
}

func TestLoadKnownKeys(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	content := `notes_dir: /tmp/notes
database: /tmp/mneme.db
review:
  tags: [foo, bar]
  match: all
  exclude: [baz]
`
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("Load known keys: %v", err)
	}
	if cfg.NotesDir != "/tmp/notes" {
		t.Errorf("NotesDir = %q, want %q", cfg.NotesDir, "/tmp/notes")
	}
	if cfg.Database != "/tmp/mneme.db" {
		t.Errorf("Database = %q, want %q", cfg.Database, "/tmp/mneme.db")
	}
	if cfg.Review.Match != "all" {
		t.Errorf("Review.Match = %q, want %q", cfg.Review.Match, "all")
	}
	if len(cfg.Review.Tags) != 2 {
		t.Errorf("Review.Tags = %v, want 2 elements", cfg.Review.Tags)
	}
}

func TestLoadUnknownKeyWarns(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yaml")
	content := "notes_dir: /tmp\nunknown_key: value\n"
	if err := os.WriteFile(f, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stderr
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	cfg, err := Load(f)
	w.Close()
	os.Stderr = old

	var buf strings.Builder
	tmp := make([]byte, 1024)
	for {
		n, e := r.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if e != nil {
			break
		}
	}

	if err != nil {
		t.Fatalf("Load with unknown key should not error: %v", err)
	}
	if cfg == nil {
		t.Fatal("cfg should not be nil")
	}
	if !strings.Contains(buf.String(), "unknown_key") {
		t.Errorf("expected warning about unknown_key in stderr, got: %q", buf.String())
	}
}

func TestExpandPathTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := ExpandPath("~/foo/bar")
	expected := filepath.Join(home, "foo/bar")
	if result != expected {
		t.Errorf("ExpandPath(~/foo/bar) = %q, want %q", result, expected)
	}
}

func TestExpandPathEnvVar(t *testing.T) {
	os.Setenv("MYTEST_DIR", "/tmp/mytest")
	defer os.Unsetenv("MYTEST_DIR")
	result := ExpandPath("$MYTEST_DIR/sub")
	if result != "/tmp/mytest/sub" {
		t.Errorf("ExpandPath($MYTEST_DIR/sub) = %q, want %q", result, "/tmp/mytest/sub")
	}
}

func TestExpandPathTildeOnly(t *testing.T) {
	home, _ := os.UserHomeDir()
	result := ExpandPath("~")
	if result != home {
		t.Errorf("ExpandPath(~) = %q, want %q", result, home)
	}
}

func TestXDGConfigPath(t *testing.T) {
	os.Unsetenv("XDG_CONFIG_HOME")
	p := XDGConfigPath()
	if !strings.HasSuffix(p, "mneme/config.yaml") {
		t.Errorf("XDGConfigPath() = %q, expected to end with mneme/config.yaml", p)
	}
}

func TestXDGConfigPathCustom(t *testing.T) {
	os.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	defer os.Unsetenv("XDG_CONFIG_HOME")
	p := XDGConfigPath()
	if p != "/tmp/xdg/mneme/config.yaml" {
		t.Errorf("XDGConfigPath() = %q, want %q", p, "/tmp/xdg/mneme/config.yaml")
	}
}

func TestXDGStatePath(t *testing.T) {
	os.Unsetenv("XDG_STATE_HOME")
	p := XDGStatePath()
	if !strings.HasSuffix(p, "mneme/mneme.db") {
		t.Errorf("XDGStatePath() = %q, expected to end with mneme/mneme.db", p)
	}
}
