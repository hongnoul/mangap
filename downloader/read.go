package downloader

import (
	"fmt"
	"os"
	"strings"

	"github.com/hongnoul/mangap/history"
	"github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/log"
	"github.com/hongnoul/mangap/reader"
	"github.com/hongnoul/mangap/reader/pages"
	"github.com/hongnoul/mangap/source"
	"github.com/spf13/viper"
)

// Prepare gets the chapter's pages ready for the built-in reader. A chapter
// already downloaded as a folder or cbz/zip is opened from disk (when
// downloader.read_downloaded is set). Anything else is downloaded into
// memory and read from there, with no converter step.
func Prepare(chapter *source.Chapter, progress func(string)) (pages.Source, error) {
	if ls, ok := chapter.Manga.Source.(source.LocalSource); ok {
		// From the downloads archive: there is nothing to fetch.
		progress("Opening")
		return pages.Open(ls.PathOf(chapter))
	}
	if viper.GetBool(key.DownloaderReadDownloaded) && chapter.IsDownloaded() {
		if path, err := chapter.Path(false); err == nil {
			src, err := pages.Open(path)
			if err == nil {
				log.Infof("reading downloaded %s", path)
				return src, nil
			}
			// e.g. a pdf export: not readable in place, fetch the pages.
			log.Infof("cannot read %s in place (%s), downloading pages", path, err)
		}
	}

	log.Infof("downloading %s for reading. Provider is %s", chapter.Name, chapter.Source().ID())
	progress("Getting pages")
	if _, err := chapter.Source().PagesOf(chapter); err != nil {
		log.Error(err)
		return nil, err
	}

	if err := chapter.DownloadPages(true, progress); err != nil {
		log.Error(err)
		return nil, err
	}

	names := make([]string, 0, len(chapter.Pages))
	data := make([][]byte, 0, len(chapter.Pages))
	for _, p := range chapter.Pages {
		if p == nil || p.Contents == nil {
			continue
		}
		names = append(names, p.Filename())
		data = append(data, p.Contents.Bytes())
	}

	src, err := pages.NewMem(names, data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", chapter.Name, err)
	}
	return src, nil
}

// ReaderOptions builds reader options from the config
// (reader.direction).
func ReaderOptions() reader.Options {
	var opt reader.Options

	if strings.ToLower(viper.GetString(key.ReaderDirection)) == "ltr" {
		rtl := false
		opt.RTL = &rtl
	}

	if os.Getenv("MANGAP_TRACE") == "1" {
		opt.Trace = os.Stderr
	}

	return opt
}

// SaveReadHistory records the chapter as read, if enabled.
func SaveReadHistory(chapter *source.Chapter) {
	// Archive chapters have no provider to resume from in history.
	if !viper.GetBool(key.HistorySaveOnRead) || source.IsLocal(chapter) {
		return
	}

	// Synchronous: a later SaveReadProgress must not be overtaken.
	if err := history.Opened(chapter); err != nil {
		log.Warn(err)
	}
}

// SaveReadProgress bookmarks where the reader stopped in chapter.
func SaveReadProgress(chapter *source.Chapter, res reader.Result) {
	if chapter == nil || !viper.GetBool(key.HistorySaveOnRead) || source.IsLocal(chapter) {
		return
	}
	if err := history.SaveProgress(chapter, res.Page, res.Pages, res.Finished); err != nil {
		log.Warn(err)
	}
}

// Read prepares the chapter and reads it in the built-in reader, which
// takes over the terminal until the user quits. Callers running a
// bubbletea program should use tea.Exec with reader.Command instead.
func Read(chapter *source.Chapter, progress func(string)) error {
	src, err := Prepare(chapter, progress)
	if err != nil {
		return err
	}
	defer src.Close()

	SaveReadHistory(chapter)
	if !reader.UseTerminal(viper.GetString(key.ReaderViewer)) {
		_, err := reader.OpenExternal(src, chapter.Manga.Name+" - "+chapter.Name)
		return err
	}
	opt, nb := ReaderOptionsFor(chapter)
	res, err := reader.Run(src, opt)
	last := chapter
	if nb != nil {
		last = nb.Current
	}
	SaveReadProgress(last, res)
	return err
}
