The first release. Ldapper opens, connects to a directory, walks its tree and
reads an object's attributes. It is strictly read-only.

## What works

- **Connections** over LDAPS, StartTLS or plain LDAP, with a simple bind or an
  NTLM bind — so you can sign in as `CORP\a.kensel` without knowing your own
  distinguished name. Passwords go to the system keychain, never to the
  settings file.
- **Untrusted certificates** stop the connection and show you the SHA-256
  fingerprint, subject, issuer and expiry. Trusting one applies to that server
  alone. The button that continues past the warning is deliberately not the
  accent colour.
- **The tree**, loaded a page at a time and virtualised, so an organizational
  unit holding tens of thousands of accounts opens rather than freezes. When
  there is more to fetch, the row says which page comes next instead of
  spinning.
- **Attributes** shown as the server stores them, with the meaning underneath:
  `objectSid` as `S-1-5-21-…`, `objectGUID` as a readable UUID, Windows
  FILETIME values as dates, and `userAccountControl` as the flags it packs.
- **18 built-in filters** for Active Directory and for plain LDAP, each marked
  with whether the server you are connected to can actually answer it. The
  library is in this build; the screen that runs filters is not yet.

## What is not here yet

Search, the filter library screen, and export to LDIF or CSV — all three are
built and tested in the engine, and are waiting on their interface. Writing to
the directory, snapshots and diffs, and Kerberos are not in v1 at all.

## Installing

**macOS** — unzip and drag `Ldapper.app` to Applications. The build is not
notarised yet, so the first launch needs right-click → Open.

**Windows** — unzip and run `Ldapper.exe`. It needs the WebView2 runtime,
which Windows 11 and current Windows 10 already have. There is no installer in
this release.

## Reporting something

The engine is covered by 272 unit tests and 27 integration tests against a
real OpenLDAP server, but no test suite has met your directory. If Ldapper
misreads an attribute or refuses a server it should accept, an issue with the
attribute name and its raw value is exactly what is useful.
