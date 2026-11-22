package provider

import (
	"testing"

	"github.com/hongnoul/mangaz/provider/mangapill"
)

func TestGet(t *testing.T) {
	if _, ok := Get(mangapill.Config.Name); !ok {
		t.Fatal("mangapill should be a builtin")
	}
	if _, ok := Get("kek"); ok {
		t.Fatal("unknown provider found")
	}
	if b := Builtins(); len(b) != 2 || b[0].Name != "Mangapill" || b[1].Name != "Mangadex" {
		t.Fatalf("builtins = %v, want [Mangapill Mangadex]", b)
	}
}
