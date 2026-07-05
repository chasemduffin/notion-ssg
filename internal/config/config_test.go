package config

import "testing"

func TestParseUsesEnvToken(t *testing.T) {
	cfg, err := Parse([]string{"--nav-root", "cmd.bio", "--output", "out"}, func(key string) string {
		if key == "NOTION_PAT" {
			return "env-token"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.NotionPAT != "env-token" {
		t.Fatalf("NotionPAT = %q", cfg.NotionPAT)
	}
}

func TestParseFlagTokenOverridesEnv(t *testing.T) {
	cfg, err := Parse([]string{"--notion-pat", "flag-token", "--nav-root", "cmd.bio", "--output", "out"}, func(string) string {
		return "env-token"
	})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if cfg.NotionPAT != "flag-token" {
		t.Fatalf("NotionPAT = %q", cfg.NotionPAT)
	}
}

func TestParseRequiresToken(t *testing.T) {
	_, err := Parse([]string{"--nav-root", "cmd.bio", "--output", "out"}, func(string) string { return "" })
	if err == nil {
		t.Fatal("expected missing token error")
	}
}
