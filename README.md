# mangaz

A terminal manga reader. Search sources, pick a chapter, and read it right in
the terminal through the kitty graphics protocol. No external viewer.

Forked from [mangal](https://github.com/metafates/mangal) (MIT, unmaintained
since April 2025), which supplies the sources, Lua scrapers, downloader, and
TUI. The in-terminal reader follows the design of pdfpane.

> **Status:** Phase 0 (rename) is done. The built-in reader is not written yet.
> See [docs/READER_PLAN.md](docs/READER_PLAN.md).

## Requirements

- A terminal with kitty graphics: kitty, ghostty, wezterm, or gwae.
- Go (version in `go.mod`) to build.

## Build

```sh
make build      # ./mangaz
make install    # into $(go env GOPATH)/bin
make test
```

## Defaults

- Right-to-left reading.
- Dark tone (`d` toggles).

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
