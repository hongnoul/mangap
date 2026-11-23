# mangap

A terminal manga reader. Search sources, pick a chapter, and read it right in
the terminal through the kitty graphics protocol. No external viewer.

Forked from [mangal](https://github.com/metafates/mangal) (MIT, unmaintained
since April 2025), which supplies the sources, Lua scrapers, downloader, and
TUI. The in-terminal reader follows the design of pdfpane.

## Requirements

A terminal with kitty graphics: kitty, ghostty, wezterm, or gwae.

## Install

```sh
# Homebrew (macOS, Linux)
brew install hongnoul/tap/mangap

# Go
go install github.com/hongnoul/mangap@latest
```

Or grab a binary for macOS, Linux, or Windows from
[Releases](https://github.com/hongnoul/mangap/releases).

## Build from source

Needs Go (version in `go.mod`).

```sh
make build      # ./mangap
make install    # into $(go env GOPATH)/bin
make test
```

## Reading

- From the TUI: pick a chapter and press the read key. Pages download into
  memory and open in the terminal. `q` goes back to the chapter list.
- Downloaded chapters: `mangap view <folder|file.cbz>`.
- Pages show as authored (no color filters). Default direction is
  right-to-left: config `reader.direction` (`rtl`/`ltr`), `D` flips it while
  reading.
- External viewers are gone. Old `reader.pdf`, `reader.cbz`, ... keys are
  ignored with a warning. `formats.use` only affects downloads now.

## Coming from mangal

On first run, mangap copies your mangal config directory (sources, history,
anilist bindings, `mangal.toml` renamed to `mangap.toml`) into the mangap
config directory. The mangal directory is left untouched. This happens only
when the mangap directory does not exist yet.

Environment variables use the `MANGAP_` prefix (was `MANGAL_`), for example
`MANGAP_CONFIG_PATH`. Run `mangap env` to list them, and `mangap where` for
paths.

## TUI keys

On list screens `j`/`k` move, `l` opens (same as enter) and `h` goes back
(same as esc). In the search box and while filtering they type normally.
Arrow keys left/right page through long lists.

## Sources

Built in: **Mangapill** (default) and **MangaDex** (fallback). Manganato and
Manganelo were removed: both return nothing now.

The search screen has a status box at the bottom showing the source,
language, and keybinds toggle. Press `ctrl+s` to pick a source (with live
RTT), `ctrl+l` to pick the MangaDex language, `ctrl+g` for keybinds.
`ctrl+o` (or `←`/`→` while the bar is empty) cycles sources.
Once you open a manga it stays on that source.

Source order and enabled set live in the config (`mangap config`):
`sources.order` and `sources.disabled`. `-S <name>` on the command line
still forces specific sources.

Custom Lua scrapers still install from
[mangal-scrapers](https://github.com/metafates/mangal-scrapers)
(`mangap sources install`). Enable one by removing it from
`sources.disabled`.

## Anilist

```
mangap integration anilist
```

Your browser opens, you click "Authorize", done. Chapters you read are then
marked as read on your Anilist list. Over SSH, add `--no-browser` and paste the
URL you land on after authorizing. To use your own API client instead of the
bundled one, create it at https://anilist.co/settings/developer with redirect
URL `http://localhost:47563/callback` and pass `--client-id <id>`. Log out with
`--disable`.

## License

MIT. See [LICENSE](LICENSE). Original work copyright (c) 2022 metafates.
