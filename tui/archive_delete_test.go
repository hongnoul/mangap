package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	key2 "github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/source"
	"github.com/spf13/viper"
)

func runeKey(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestArchiveDeleteMangas(t *testing.T) {
	a := t.TempDir()
	for _, n := range []string{"A", "B", "C"} {
		mkManga(t, a, n)
	}
	viper.Set(key2.DownloaderPath, a)
	b := newBubble()
	b.resize(100, 40)
	b.state = searchState
	b.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	b.Update(runeKey("d"))
	if !b.del.on || !strings.Contains(b.View(), "from disk?") {
		t.Fatal("d should prompt")
	}
	b.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if b.del.on || len(b.mangasC.Items()) != 3 {
		t.Fatal("esc should cancel")
	}
	b.Update(runeKey("v"))
	b.View()
	b.Update(runeKey("j"))
	b.Update(runeKey("d"))
	if len(b.del.mangas) != 2 {
		t.Fatalf("want 2 marked, got %d", len(b.del.mangas))
	}
	b.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if b.del.on || len(b.mangasC.Items()) != 1 {
		t.Fatalf("items %d err %s", len(b.mangasC.Items()), b.del.err)
	}
	es, _ := os.ReadDir(a)
	if len(es) != 1 {
		t.Fatalf("left %d", len(es))
	}
}

func TestArchiveDeleteChapter(t *testing.T) {
	a := t.TempDir()
	mkManga(t, a, "A")
	p := filepath.Join(a, "A", "Chapter 2")
	os.MkdirAll(p, 0o755)
	os.WriteFile(filepath.Join(p, "001.jpg"), []byte{0xff, 0xd8}, 0o644)
	viper.Set(key2.DownloaderPath, a)
	b := newBubble()
	b.resize(100, 40)
	b.state = searchState
	b.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m := b.mangasC.SelectedItem().(*listItem).internal.(*source.Manga)
	b.selectedManga = m
	chs, _ := m.Source.ChaptersOf(m)
	b.showChapters(chs)
	if !b.inArchiveChapters() || len(chs) != 2 {
		t.Fatalf("not in archive chapters (%d)", len(chs))
	}
	b.Update(runeKey("d"))
	b.Update(runeKey("y"))
	if len(b.chaptersC.Items()) != 1 {
		t.Fatalf("chapters %d err %s", len(b.chaptersC.Items()), b.del.err)
	}
}
