package tui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hongnoul/mangap/downloader"
	"github.com/hongnoul/mangap/history"
	"github.com/hongnoul/mangap/installer"
	"github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/log"
	"github.com/hongnoul/mangap/provider"
	"github.com/hongnoul/mangap/reader/pages"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/util"
	"github.com/spf13/viper"
	"strings"
	"sync"
)

func (b *statefulBubble) loadScrapers() tea.Cmd {
	return func() tea.Msg {
		b.progressStatus = "Loading scrapers"
		scrapers, err := installer.Scrapers()
		if err != nil {
			log.Error(err)
			b.errorChannel <- err
			return nil
		}
		b.progressStatus = "Scrapers Loaded"

		util.SortFunc(scrapers, func(a, b *installer.Scraper) bool {
			return strings.Compare(a.Name, b.Name) < 0
		})

		var items = make([]list.Item, len(scrapers))
		for i, s := range scrapers {
			items[i] = &listItem{
				internal: s,
			}
		}

		cmd := b.scrapersInstallC.SetItems(items)
		b.scrapersLoadedChannel <- scrapers
		return cmd
	}
}

func (b *statefulBubble) waitForScrapersLoaded() tea.Cmd {
	return func() tea.Msg {
		select {
		case res := <-b.scrapersLoadedChannel:
			return res
		case err := <-b.errorChannel:
			b.lastError = err
			return err
		}
	}
}

func (b *statefulBubble) installScraper(s *installer.Scraper) tea.Cmd {
	return func() tea.Msg {
		b.progressStatus = fmt.Sprintf("Installing %s", s.Name)
		err := s.Install()
		if err != nil {
			log.Error(err)
			b.errorChannel <- err
		} else {
			log.Info("scraper " + s.Name + " installed")
			b.scraperInstalledChannel <- s
		}

		return nil
	}
}

func (b *statefulBubble) waitForScraperInstallation() tea.Cmd {
	return func() tea.Msg {
		select {
		case res := <-b.scraperInstalledChannel:
			return res
		case err := <-b.errorChannel:
			b.lastError = err
			return err
		}
	}
}

func (b *statefulBubble) loadSources(ps []*provider.Provider) tea.Cmd {
	return func() tea.Msg {
		var (
			sources = make([]source.Source, len(ps))
			wg      = sync.WaitGroup{}
			err     error
		)

		wg.Add(len(ps))
		for i, p := range ps {
			go func(i int, p *provider.Provider) {
				defer wg.Done()

				if err != nil {
					return
				}

				log.Info("loading source " + p.ID)
				b.progressStatus = "Initializing source"
				var s source.Source
				s, err = p.CreateSource()

				if err != nil {
					log.Error(err)
					b.errorChannel <- err
					return
				}

				log.Info("source " + p.ID + " loaded")
				sources[i] = s
			}(i, p)
		}

		wg.Wait()

		b.sourcesLoadedChannel <- sources

		return nil
	}
}

func (b *statefulBubble) waitForSourcesLoaded() tea.Cmd {
	return func() tea.Msg {
		select {
		case res := <-b.sourcesLoadedChannel:
			return res
		case err := <-b.errorChannel:
			b.lastError = err
			return err
		}
	}
}

func (b *statefulBubble) searchManga(query string) tea.Cmd {
	return func() tea.Msg {
		log.Info("searching for " + query)
		b.progressStatus = "Searching " + autoChain(b.selectedSources)

		var mangas = make([]*source.Manga, 0)

		wg := sync.WaitGroup{}
		wg.Add(len(b.selectedSources))
		for _, s := range b.selectedSources {
			go func(s source.Source) {
				defer wg.Done()
				sourceMangas, err := s.Search(query)

				if err != nil {
					log.Error(err)
					b.errorChannel <- err
				}

				log.Infof("found %s from source %s", util.Quantify(len(sourceMangas), "manga", "mangas"), s.Name())
				mangas = append(mangas, sourceMangas...)
			}(s)
		}

		wg.Wait()

		log.Infof("found %d mangas from %d sources", len(mangas), len(b.selectedSources))

		b.foundMangasChannel <- mangas

		return nil
	}
}

func (b *statefulBubble) waitForMangas() tea.Cmd {
	return func() tea.Msg {
		select {
		case found := <-b.foundMangasChannel:
			return found
		case err := <-b.errorChannel:
			b.lastError = err
			return err
		}
	}
}

func (b *statefulBubble) getChapters(manga *source.Manga) tea.Cmd {
	return func() tea.Msg {
		log.Info("getting chapters of " + manga.Name)
		chapters, err := manga.Source.ChaptersOf(manga)
		if err != nil {
			log.Error(err)
			b.errorChannel <- err
		} else {
			log.Infof("found %s", util.Quantify(len(chapters), "chapter", "chapters"))
			b.foundChaptersChannel <- chapters
		}

		return nil
	}
}

func (b *statefulBubble) waitForChapters() tea.Cmd {
	return func() tea.Msg {
		select {
		case found := <-b.foundChaptersChannel:
			return found
		case err := <-b.errorChannel:
			b.lastError = err
			return err
		}
	}
}

func (b *statefulBubble) readChapter(chapter *source.Chapter) tea.Cmd {
	return func() tea.Msg {
		b.currentDownloadingChapter = chapter
		src, err := downloader.Prepare(chapter, func(s string) {
			b.progressStatus = s
		})

		if err != nil {
			b.errorChannel <- err
		} else {
			b.chapterReadChannel <- readyToReadMsg{chapter: chapter, src: src}
		}

		return nil
	}
}

func (b *statefulBubble) waitForChapterRead() tea.Cmd {
	return func() tea.Msg {
		select {
		case res := <-b.chapterReadChannel:
			return res
		case err := <-b.errorChannel:
			b.lastError = err
			return err
		}
	}
}

func (b *statefulBubble) downloadChapter(chapter *source.Chapter) tea.Cmd {
	return func() tea.Msg {
		b.currentDownloadingChapter = chapter
		_, err := downloader.Download(chapter, func(s string) {
			b.progressStatus = s
		})

		if err != nil {
			if viper.GetBool(key.DownloaderStopOnError) {
				b.errorChannel <- err
			} else {
				b.failedChapters = append(b.failedChapters, chapter)
				b.chapterDownloadChannel <- struct{}{}
			}
		} else {
			b.succededChapters = append(b.succededChapters, chapter)
			b.chapterDownloadChannel <- struct{}{}
		}

		return nil
	}
}

func (b *statefulBubble) waitForChapterDownload() tea.Cmd {
	return func() tea.Msg {
		select {
		case res := <-b.chapterDownloadChannel:
			return res
		case err := <-b.errorChannel:
			b.lastError = err
			return err
		}
	}
}

func (b *statefulBubble) selectChapterBy(f func(chapter *source.Chapter) bool) tea.Cmd {
	return func() tea.Msg {
		for i, item := range b.chaptersC.Items() {
			chapter := item.(*listItem).internal.(*source.Chapter)
			if f(chapter) {
				b.chaptersC.Select(i)
				return nil
			}
		}

		return nil
	}
}

// readyToReadMsg carries a prepared chapter to the read state, which hands
// it to the built-in reader through tea.Exec.
type readyToReadMsg struct {
	chapter *source.Chapter
	src     pages.Source
}

// readFinishedMsg is sent when the reader returns the terminal.
type readFinishedMsg struct {
	err     error
	chapter *source.Chapter // the chapter the reader was in when it quit
}

// selectBookmark moves the chapter cursor to where the user left off.
func (b *statefulBubble) selectBookmark() tea.Cmd {
	saved, ok := history.Lookup(b.selectedManga)
	if !ok {
		return nil
	}
	b.markBookmark(saved)
	return b.selectChapterBy(saved.Matches)
}

// markBookmark labels the bookmarked chapter and clears the others.
func (b *statefulBubble) markBookmark(saved *history.SavedChapter) {
	for _, item := range b.chaptersC.Items() {
		li := item.(*listItem)
		li.bookmark = ""
		c, ok := li.internal.(*source.Chapter)
		if !ok || !saved.Matches(c) {
			continue
		}
		switch {
		case saved.Finished:
			li.bookmark = "read"
		case saved.Pages > 0:
			li.bookmark = fmt.Sprintf("p.%d/%d", saved.Page+1, saved.Pages)
		default:
			li.bookmark = fmt.Sprintf("p.%d", saved.Page+1)
		}
	}
}
