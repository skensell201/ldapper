Three bugs found by pointing Ldapper at a directory built to be awkward rather
than tidy. All three are the kind that only show up against real data.

## Fixed

- **A comma inside a name was shown as its escape sequence.** `Volkova, Anna`
  came back from the server as `Volkova\2C Anna` — or `Volkova\, Anna`,
  depending on the server — and the tree showed it that way. Names are now
  unescaped for display while the distinguished name keeps its escapes, which
  is what makes it usable as an identifier.
- **CSV could not be read back reliably.** Multiple values were joined with
  `"; "`, so a single value that happened to contain a semicolon looked
  identical to several values. Values are now separated by newlines inside the
  quoted field, which a spreadsheet shows as line breaks and a CSV reader
  parses back exactly.
- **The built-in "empty groups" filter never matched anything.** It looked only
  at `group` and `groupOfNames`, and both require a member by schema — a
  genuinely empty group is almost always a `posixGroup`. The filter now covers
  that case, and `uniqueMember` besides.

## Also in this release

`dev/directory/` — a generator and a compose file for a directory worth
exploring: 2594 entries, groups of 3 to 500 members, POSIX accounts, 60
computers, and the awkward cases that found the bugs above. A comma inside a
name, Cyrillic in every field, semicolons inside a value, a 900-character
value, and an entry missing an attribute the table has a column for.

```bash
docker compose -f dev/directory/docker-compose.yml up -d --wait
```

Then connect to `localhost:4389` as `cn=admin,dc=example,dc=com` with the
password `adminpassword`.

## Verified

311 unit tests and 37 integration tests, the latter now including an entry
whose name contains a comma — checked all the way from the server to the label
the interface would draw. Everything above was also confirmed by hand against
the development directory: 2508 people paged in three round trips, a
500-member group exported in full, a 900-character value surviving LDIF, and
Cyrillic base64-encoded correctly.
