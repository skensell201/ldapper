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

Pre-implementation. The design is settled and written down:

- Design spec — [`docs/superpowers/specs/2026-08-05-ldapper-design.md`](docs/superpowers/specs/2026-08-05-ldapper-design.md)
- Visual mockups — [`docs/design/`](docs/design/) (open the HTML files in a browser)

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
