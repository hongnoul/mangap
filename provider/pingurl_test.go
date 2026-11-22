package provider

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCustomPingURL(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"-- @url     https://weebcentral.com\nlocal x = 1": "https://weebcentral.com",
		"local BASE = \"https://example.org/path\"":        "https://example.org",
		"print('hi')": "",
	}
	i := 0
	for src, want := range cases {
		p := filepath.Join(dir, string(rune('a'+i))+".lua")
		i++
		_ = os.WriteFile(p, []byte(src), 0o644)
		if got := customPingURL(p); got != want {
			t.Errorf("%q: got %q want %q", src, got, want)
		}
	}
}
