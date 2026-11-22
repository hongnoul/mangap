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
- Defaults: right-to-left, dark tone. Config `reader.direction` (`rtl`/`ltr`)
  and `reader.tone` (`dark`/`light`/`dim`); `D` and `d` change them while
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

## Sources

Built in: MangaDex, Manganelo, Manganato, Mangapill. Custom Lua scrapers still
install from [mangal-scrapers](https://github.com/metafates/mangal-scrapers)
(`mangaz sources install`).

## License

MIT. See [LICENSE](LICENSE). Original work copyright (c) 2022 metafates.
