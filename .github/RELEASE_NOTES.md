Ldapper has a new look, and now a light one as well as a dark one.

## Light and dark

The window follows the system. In light mode it is a pale grey ground with
white cards lifted off it by a soft shadow; in dark mode the same layout sits
on graphite, with hairline edges where a shadow would not show. There is no
setting to find — change the system appearance and Ldapper changes with it.

## Ink and three tones

The one thing to press on a screen — Connect, Search, Save, Copy DN — is an
ink-black pill in light mode and a white one in dark. Everything else is an
outlined pill or plain text, so the eye still finds the button that matters.

Colour now marks what a thing is rather than how important it is:

- **Lilac** is the interface pointing: the selected row, pane headings, the
  selected saved connection, the edited mark on a filter.
- **Mint** is time and health: decoded timestamps, `never`, and a working
  connection.
- **Rose** is flags: the bits decoded from `userAccountControl`.

Decoded values are token chips toned that way, so in an attribute table a SID,
a date and a flag can be told apart before they are read. Filter dialects get
the same treatment in the library — Active Directory, generic and POSIX each
have their own tone.

## Roomier

Tree rows, result rows and attribute rows are a little taller, headings a
little larger, and fields and buttons a size up. The status bar shows each
fact about the connection as its own chip.

## Fixed

- In the connection dialog, "Remember the password" was set in monospace and
  wrapped mid-word: it was picking up a style meant for the filter editor.

## Nothing else changed

This release is visual only. No behaviour, no settings and no saved
connections are touched.

## Verified

318 Go tests, 56 frontend tests and 37 integration tests. The screenshots in
the README are retaken from this build's interface.
