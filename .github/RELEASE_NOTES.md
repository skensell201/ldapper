Search, the filter library and export. With these, everything the v1 design
called for is in place: read, search, filter, export. Still strictly
read-only.

## New in this release

- **Search** streams as it runs. Results appear while the server is still
  walking the tree rather than after it finishes, and Stop halts a search
  partway and says how far it got. A filter is compiled locally first, so a
  malformed one never reaches the server — where the answer would be a
  protocol error that says nothing about which part was wrong.
- **The filter library** with its editor. All 18 built-in filters are
  editable, a changed one is marked, and Reset returns it to the version
  Ldapper ships. Deleting a built-in makes it stay deleted across updates.
  Filters the connected server cannot answer are greyed with the reason,
  never hidden — a greyed row tells you something, a missing one does not.
- **Substitutions resolve as you type.** `{{now-90d:filetime}}` shows the
  18-digit Windows FILETIME it becomes before you run anything.
- **Export to LDIF and CSV**, written straight to the file you choose. A
  subtree export never passes through the interface, which is exactly the case
  that would run out of memory if it did.

## Carried over from v0.1.0

Connections over LDAPS, StartTLS or plain LDAP with a simple or NTLM bind;
passwords in the system keychain; untrusted certificates shown with their
fingerprint before you decide; a paged, virtualised tree; and attributes
decoded underneath their raw values.

## What is still not here

Writing to the directory, snapshots and diffs, and Kerberos — none of them are
in v1 by design. Reconnecting automatically after a dropped connection is
deferred: the event bridge it needs exists now, but there is no way yet to
produce a dropped connection on purpose to test against.

## Installing

**macOS** — unzip and drag `Ldapper.app` to Applications. Not notarised, so
the first launch needs right-click → Open.

**Windows** — unzip and run `Ldapper.exe`. Needs the WebView2 runtime, which
current Windows 10 and 11 already carry. No installer yet.

## What is verified, and what is not

301 unit tests and 36 integration tests against a real OpenLDAP server, the
latter driving the same facade the window calls: connect, page through a
branch, read an entry, validate and expand a filter, export to both formats.
The window itself has been run and its chrome checked by eye on macOS. The
Windows build compiles and links in CI but has not been run by hand — if it
misbehaves there, that is the report most worth having.
