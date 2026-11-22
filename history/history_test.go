package history

import (
	"fmt"
	"github.com/hongnoul/mangaz/filesystem"
	"github.com/hongnoul/mangaz/source"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

type testSource struct{}

func (testSource) Name() string {
	panic("")
}

func (testSource) Search(_ string) ([]*source.Manga, error) {
	panic("")
}

func (testSource) ChaptersOf(_ *source.Manga) ([]*source.Chapter, error) {
	panic("")
}

func (testSource) PagesOf(_ *source.Chapter) ([]*source.Page, error) {
	panic("")
}

func (testSource) ID() string {
	return "test source"
}

func init() {
	filesystem.SetMemMapFs()
}

func TestHistory(t *testing.T) {
	Convey("Given a chapter", t, func() {
		chapter := source.Chapter{
			Name:  "adwad",
			URL:   "dwaofa",
			Index: 42069,
			ID:    "fawfa",
			Pages: nil,
		}
		manga := source.Manga{
			Name:     "dawf",
			URL:      "fwa",
			Index:    1337,
			ID:       "wjakfkawgjj",
			Source:   testSource{},
			Chapters: []*source.Chapter{&chapter},
		}
		chapter.Manga = &manga

		Convey("When saving the chapter", func() {
			err := Save(&chapter)
			Convey("Then the error should be nil", func() {
				So(err, ShouldBeNil)

				Convey("And the chapter should be saved", func() {
					chapters, err := Get()
					So(err, ShouldBeNil)
					So(len(chapters), ShouldBeGreaterThan, 0)
					So(chapters[fmt.Sprintf("%s (%s)", chapter.Manga.Name, chapter.Source().ID())].Name, ShouldEqual, chapter.Name)
				})
			})
		})
	})
}

func TestProgress(t *testing.T) {
	Convey("Given a bookmarked chapter", t, func() {
		ch := source.Chapter{Name: "Ch 2", URL: "u2", ID: "c2"}
		other := source.Chapter{Name: "Ch 3", URL: "u3", ID: "c3"}
		manga := source.Manga{Name: "progress manga", Source: testSource{}, Chapters: []*source.Chapter{&ch, &other}}
		ch.Manga, other.Manga = &manga, &manga

		So(SaveProgress(&ch, 12, 38, false), ShouldBeNil)

		Convey("Lookup returns the exact page", func() {
			saved, ok := Lookup(&manga)
			So(ok, ShouldBeTrue)
			So(saved.Page, ShouldEqual, 12)
			So(saved.Pages, ShouldEqual, 38)
			So(saved.ResumePage(&ch), ShouldEqual, 12)
			So(saved.ResumePage(&other), ShouldEqual, 0)
		})

		Convey("Reopening the same chapter keeps the page", func() {
			So(Opened(&ch), ShouldBeNil)
			saved, _ := Lookup(&manga)
			So(saved.Page, ShouldEqual, 12)
		})

		Convey("Opening another chapter resets it", func() {
			So(Opened(&other), ShouldBeNil)
			saved, _ := Lookup(&manga)
			So(saved.URL, ShouldEqual, "u3")
			So(saved.Page, ShouldEqual, 0)
		})

		Convey("A finished chapter resumes at 0", func() {
			So(SaveProgress(&ch, 37, 38, true), ShouldBeNil)
			saved, _ := Lookup(&manga)
			So(saved.ResumePage(&ch), ShouldEqual, 0)
		})
	})
}
