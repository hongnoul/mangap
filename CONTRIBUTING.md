# Contributing

Thanks for helping. Issues and PRs are both welcome.

## Setup

```sh
git clone https://github.com/hongnoul/mangap
cd mangap
make build   # ./mangap
make test
```

You need Go (version in `go.mod`) and a terminal with kitty graphics
(kitty, Ghostty, WezTerm, or gwae) to try the reader.

## Layout

| Path | What lives there |
| ---- | ---------------- |
| `tui/` | Bubble Tea UI: search, manga grid, chapters, downloads viewer |
| `reader/` | In-terminal reader: kitty graphics, input, page rendering |
| `provider/` | Built-in sources (Mangapill, MangaDex) |
| `source/` | Source interface shared by built-in and Lua sources |
| `downloader/`, `converter/` | Downloads and pdf/cbz/zip/plain output |
| `examples/` | Example Lua scraper |

## Adding a source

The quickest route is a Lua scraper. `mangap sources gen` writes a
template, and `examples/WeebCentral.lua` is a complete working one. Drop
the file in the directory from `mangap where --sources` and it shows up in
the source picker.

Built-in Go sources live in `provider/`. Open a
[source request](https://github.com/hongnoul/mangap/issues/new?template=source_request.yaml)
first so we can check the site works and is SFW.

## Pull requests

- Keep PRs focused. One fix or feature each.
- `go test ./...` passes. CI runs it on Linux, macOS, and Windows.
- Add a test when you fix a bug.
- Commit messages: `area: what changed`, for example
  `reader: keep scroll position after resize`.

Look for issues labeled
[good first issue](https://github.com/hongnoul/mangap/labels/good%20first%20issue)
if you want somewhere to start.
