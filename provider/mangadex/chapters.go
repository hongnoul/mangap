package mangadex

import (
	"fmt"
	"github.com/darylhjd/mangodex"
	"github.com/hongnoul/mangap/key"
	"github.com/hongnoul/mangap/source"
	"github.com/hongnoul/mangap/util"
	"github.com/spf13/viper"
	"net/url"
	"strconv"
)

func (m *Mangadex) ChaptersOf(manga *source.Manga) ([]*source.Chapter, error) {
	language := viper.GetString(key.MangadexLanguage)
	// Cache per language so switching languages never serves stale chapters.
	cacheKey := manga.URL + "|" + language
	if cached, ok := m.cache.chapters.Get(cacheKey).Get(); ok {
		for _, chapter := range cached {
			chapter.Manga = manga
		}

		return cached, nil
	}

	params := url.Values{}
	params.Set("limit", strconv.Itoa(500))
	ratings := []string{mangodex.Safe, mangodex.Suggestive}
	for _, rating := range ratings {
		params.Add("contentRating[]", rating)
	}

	if viper.GetBool(key.MangadexNSFW) {
		params.Add("contentRating[]", mangodex.Porn)
		params.Add("contentRating[]", mangodex.Erotica)
	}

	// scanlation group for the chapter
	params.Add("includes[]", mangodex.ScanlationGroupRel)
	params.Set("order[chapter]", "asc")
	if filterLanguage(language) {
		// Filter server side. Client side filtering paged incorrectly.
		params.Add("translatedLanguage[]", language)
	}

	var (
		chapters   []*source.Chapter
		currOffset = 0
	)

	for {
		params.Set("offset", strconv.Itoa(currOffset))
		list, err := m.client.Chapter.GetMangaChapters(manga.ID, params)
		if err != nil {
			return nil, err
		}

		for i, chapter := range list.Data {
			// Skip external chapters. Their pages cannot be downloaded.
			if chapter.Attributes.ExternalURL != nil && !viper.GetBool(key.MangadexShowUnavailableChapters) {
				continue
			}

			if filterLanguage(language) && chapter.Attributes.TranslatedLanguage != language {
				continue
			}

			name := chapter.GetTitle()
			if name == "" {
				name = fmt.Sprintf("Chapter %s", chapter.GetChapterNum())
			} else {
				name = fmt.Sprintf("Chapter %s - %s", chapter.GetChapterNum(), name)
			}

			var volume string
			if chapter.Attributes.Volume != nil {
				volume = fmt.Sprintf("Vol.%s", *chapter.Attributes.Volume)
			}
			chapters = append(chapters, &source.Chapter{
				Name:   name,
				Index:  uint16(i),
				ID:     chapter.ID,
				URL:    fmt.Sprintf("https://mangadex.org/chapter/%s", chapter.ID),
				Manga:  manga,
				Volume: volume,
			})
		}
		currOffset += 500
		if currOffset >= list.Total {
			break
		}
	}

	util.SortFunc(chapters, func(a, b *source.Chapter) bool {
		return a.Index < b.Index
	})

	manga.Chapters = chapters
	_ = m.cache.chapters.Set(cacheKey, chapters)
	return chapters, nil
}
