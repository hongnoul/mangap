# mangaz

A terminal manga reader. Search sources, pick a chapter, and read it right in
the terminal through the kitty graphics protocol. No external viewer.

Forked from [mangal](https://github.com/metafates/mangal) (MIT, unmaintained
since April 2025), which supplies the sources, Lua scrapers, downloader, and
TUI. The in-terminal reader follows the design of pdfpane.

> **Status:** Phases 0-2 are done. Reading a chapter from the TUI (or
> `mangaz mini` / `inline`) opens it in the built-in reader; `q` returns to
> the chapter list. See [docs/READER_PLAN.md](docs/READER_PLAN.md).

## Requirements

- A terminal with kitty graphics: kitty, ghostty, wezterm, or gwae.
- Go (version in `go.mod`) to build.

## Build

```sh
make build      # ./mangaz
make install    # into $(go env GOPATH)/bin
make test
```

## Reading

- From the TUI: pick a chapter and press the read key. Pages download into
  memory and open in the terminal. `q` goes back to the chapter list.
- Downloaded chapters: `mangaz view <folder|file.cbz>`.
- Pages show as authored (no color filters). Default direction is
  right-to-left: config `reader.direction` (`rtl`/`ltr`), `D` flips it while
  reading.
- External viewers are gone. Old `reader.pdf`, `reader.cbz`, ... keys are
  ignored with a warning. `formats.use` only affects downloads now.

## Coming from mangal

On first run, mangaz copies your mangal config directory (sources, history,
anilist bindings, `mangal.toml` renamed to `mangaz.toml`) into the mangaz
config directory. The mangal directory is left untouched. This happens only
when the mangaz directory does not exist yet.

Environment variables use the `MANGAZ_` prefix (was `MANGAL_`), for example
`MANGAZ_CONFIG_PATH`. Run `mangaz env` to list them, and `mangaz where` for
paths.

## TUI keys

On list screens `j`/`k` move, `l` opens (same as enter) and `h` goes back
(same as esc). In the search box and while filtering they type normally.
Arrow keys left/right page through long lists.

## Sources

Built in: **Mangapill** (default) and **MangaDex** (fallback). Manganato and
Manganelo were removed: both return nothing now.

By default the TUI skips the source picker and goes straight to search
(`sources.mode = auto`). Each search tries the sources in `sources.order`
and shows the first one with results; the list title says which one
answered. A source that errors, finds nothing, or takes over 8s passes to
the next. Once you open a manga it stays on that source.

Press `ctrl+s` on the search screen for the source settings:

- `j`/`k` move, `J`/`K` reorder, `space` enable/disable, `m` switch
  auto/manual, `enter` save, `esc` cancel.
- Manual mode brings back the old picker on start.
- `-S <name>` on the command line still forces specific sources.

Custom Lua scrapers still install from
[mangal-scrapers](https://github.com/metafates/mangal-scrapers)
(`mangaz sources install`). They show up in the settings screen, disabled
until you turn them on.

## License

MIT. See [LICENSE](LICENSE). Original work copyright (c) 2022 metafates.
