package theme

import "testing"

func TestLoadBuiltInSPATheme(t *testing.T) {
	th, err := Load("cmd-bio")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if th.Mode != ModeSPA {
		t.Fatalf("Mode = %q", th.Mode)
	}
}

func TestLoadRejectsUnknownTheme(t *testing.T) {
	_, err := Load("does-not-exist.yaml")
	if err == nil {
		t.Fatal("expected error")
	}
}
