<img src="assets/icon.svg" width="88" alt="">

# Ldapper

A desktop explorer for LDAP directories — a replacement for Sysinternals AD Explorer that also
speaks plain LDAP.

Ldapper walks the directory tree, shows an object's attributes in full, runs raw RFC 4515 filters
against it, and exports what you find. Version 1 is strictly read-only.

## Why another one

Every attribute value is shown as the server returned it — and decoded right underneath.
`objectSid` in base64 tells you nothing; `S-1-5-21-3638186439-3476304828-32309524-1104` does.
The same goes for `objectGUID`, Windows FILETIME timestamps, and the bit flags packed into
`userAccountControl`.

## Status

The engine is built and tested; the interface is not started yet. `ldapper-probe`
drives the whole engine from a terminal:

```bash
go build ./cmd/ldapper-probe
./ldapper-probe -host dc01.corp.example.com -port 636 -encryption ldaps \
  -ntlm -user 'CORP\a.kensel' -password '…' info
```

`info` reports what the server is, `browse` walks a branch page by page,
`search` runs a filter and can export the results, and `filters` lists the
built-in set without connecting to anything.

The design is settled and written down:

- Design spec — [`docs/superpowers/specs/2026-08-05-ldapper-design.md`](docs/superpowers/specs/2026-08-05-ldapper-design.md)
- Implementation plan for the engine — [`docs/superpowers/plans/2026-08-05-ldapper-core.md`](docs/superpowers/plans/2026-08-05-ldapper-core.md)
- Visual mockups and the logo — [`docs/design/`](docs/design/) (open the HTML files in a browser)

## The mark

Every directory listing draws its indentation with `└` — which is also the first letter of the
name. The logo is two levels of nesting and three nodes, with the leaf in coral: you descend the
branch, and the thing you were looking for lights up.

Source of truth is `assets/icon.svg`. Regenerate the raster sizes from it rather than editing them:

```bash
rsvg-convert -w 1024 -h 1024 assets/icon.svg -o build/appicon.png
```

## Tests

```bash
make test         # unit tests, race detector on
make lint         # go vet and golangci-lint
make integration  # starts OpenLDAP in Docker, runs against it, tears it down
```

The integration fixture seeds 122 people so that a page size of 50 needs three
round trips. If the paging control ever stops reaching the server, that test
reports one page instead of three — a smaller fixture would pass silently and
browsing a real organizational unit would quietly stop at its first thousand
objects.

## Planned for v1

- Directory tree with paged loading, built for organizational units holding tens of thousands of objects
- Full attribute view with decoded SIDs, GUIDs, timestamps and account flags
- Raw LDAP filter search, streamed as results arrive
- 18 built-in filters — every one of them editable, with a reset to the shipped version
- Connections over LDAPS or StartTLS, simple bind or NTLM, passwords held in the OS keychain
- Export to LDIF and CSV

Not in v1: writing to the directory, snapshots and diffs, Kerberos, schema editing.

## Built with

Go and [go-ldap](https://github.com/go-ldap/ldap) for the core, [Wails](https://wails.io) for the
shell, React and TypeScript for the interface. Windows is the primary target; macOS and Linux
builds follow.

## License

MIT
