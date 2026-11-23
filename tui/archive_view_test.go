package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	key2 "github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/where"
	"github.com/spf13/viper"
)

func mkManga(t *testing.T, dir, name string) {
	p := filepath.Join(dir, name, "Chapter 1")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(p, "001.jpg"), []byte{0xff, 0xd8}, 0o644)
}

func TestArchiveFilterAndDir(t *testing.T) {
	a, c := t.TempDir(), t.TempDir()
	mkManga(t, a, "One Piece")
	mkManga(t, a, "Naruto")
	mkManga(t, c, "Bleach")
	viper.Set(key2.DownloaderPath, a)
	viper.SetConfigFile(filepath.Join(t.TempDir(), "mangap.toml"))
	b := newBubble()
	b.resize(100, 40)
	b.state = searchState
	b.Update(tea.KeyMsg{Type: tea.KeyCtrlJ})
	if !b.inArchive() || len(b.mangasC.Items()) != 2 {
		t.Fatalf("archive items %d", len(b.mangasC.Items()))
	}
	b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}) // browse
	if b.arch.typing || b.mangasC.Index() != 1 {
		t.Fatalf("j should move, typing=%v idx=%d", b.arch.typing, b.mangasC.Index())
	}
	b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	for _, r := range "naru" {
		b.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if n := len(b.mangasC.Items()); n != 1 {
		t.Fatalf("filtered %d", n)
	}
	if v := b.View(); !strings.Contains(v, "Save dir") || !strings.Contains(v, "1/2") {
		t.Fatalf("view:\n%s", v)
	}
	b.Update(tea.KeyMsg{Type: tea.KeyEsc}) // leaves and clears filter
	if len(b.mangasC.Items()) != 2 || !b.inArchive() || b.arch.typing {
		t.Fatal("esc should clear filter")
	}
	b.Update(tea.KeyMsg{Type: tea.KeyCtrlE})
	b.arch.dir.SetValue(c)
	b.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if where.Downloads() != c || len(b.mangasC.Items()) != 1 {
		t.Fatalf("dir %s items %d err %s", where.Downloads(), len(b.mangasC.Items()), b.arch.dirErr)
	}
	t.Log("\n" + b.View())
}
