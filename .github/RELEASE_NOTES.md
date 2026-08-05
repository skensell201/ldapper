The interface is now tested, not just built — and there is a single command
that opens Ldapper against a real directory with nothing to fill in.

## The window has tests now

Twenty-four of them, rendering every pane with the shapes the Go side actually
produces. This closes the gap that mattered most: until now the panes were
verified by reading them, and the one time that was not enough, the
application opened to an empty rectangle.

They cover what would break silently — a pane crashing on real data, a
decoded value not reaching the screen, a search batch arriving from Go and not
appearing, a truncated search reported as "found nothing", a filter the server
cannot answer being hidden instead of greyed.

## Connect without an account

Plenty of directories allow reading without credentials, and Ldapper had no
way to ask. **Anonymous** joins NTLM and simple bind in the connection dialog,
and the credential fields step aside when it is chosen.

## The connection dialog lists your connections

It had no way to pick among saved connections at all: it silently edited the
first one and read its values once, so opening it a moment early showed
factory defaults over a connection that was already there — with nothing on
screen to explain it. There is now a list of saved connections with a **+ New**
beside them.

## One command to see it

```bash
make demo
```

A directory of 2594 entries in Docker, a connection saved for it, and the
application open and pointed at it. Press Connect. `make demo-down` when done.

## Also fixed

- A binding that returns nothing — the Go side failing in a way it could not
  report — left the window silently empty. It now says the directory did not
  answer.
- Errors while loading saved connections were swallowed, producing the same
  unexplained empty form.

## Verified

313 Go tests, 24 frontend tests, 37 integration tests against OpenLDAP. The
Windows build compiles and links in CI but still has not been run by hand.
