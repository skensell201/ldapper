A connection's health now has a colour, and the title bar carries the mark.

## Green, amber, red

The rest of the interface is deliberately aubergine and coral, but a
connection's health is not decoration — it is the one thing worth reading from
across the room.

- **Green** — connected over LDAPS or StartTLS, working.
- **Amber** — connected, but without TLS. Credentials are crossing the network
  in the clear. It works, and you should know.
- **Red** — the server could not be reached, or something went wrong since.

Both colours are pulled toward the aubergine so they belong to this interface
rather than looking like a traffic light bolted onto it. One function decides
which state applies, and the indicator in the title bar and the status bar
both read it — so they cannot disagree about what the dot means.

The status bar spells the state out beside the dot, and calls out an
unencrypted connection as `no TLS` rather than stating it flatly.

## The title bar

It carries the Ldapper mark now, beside the name.

The left inset that macOS needs for its window controls was being applied
everywhere, which pushed the name off centre on Windows. Only macOS gets it.

## Verified

315 Go tests, 41 frontend tests, 37 integration tests. The health states have
tests of their own, including the one that matters most: an error outranks an
open connection, because saying "Connected" over the top of a failure would be
a lie.
