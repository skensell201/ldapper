# A directory to explore

`test/integration` has a fixture that is deliberately small and boring: it
exists so tests can assert exact numbers. This one is for driving Ldapper by
hand — a plausible company with the awkward cases a real directory
accumulates.

```bash
docker compose -f dev/directory/docker-compose.yml up -d --wait
```

Then connect Ldapper to `localhost:4389`, no encryption, simple bind as
`cn=admin,dc=example,dc=com` with the password `adminpassword`.

## What is in it

2594 entries, including:

- 2500 people under `ou=people,ou=corp` — enough that a page size of 1000
  needs three round trips
- Groups of 3, 40, 120 and 500 members, so a small membership list and a large
  one can be compared
- An actually empty group, which is what the built-in filter looks for
- Three POSIX accounts, so the POSIX dialect has something to find
- 60 computers, five department branches, four service accounts

## The awkward ones

These are the reason this file exists. Each is a case that has broken a
directory browser somewhere:

| Entry | What it tests |
|---|---|
| `cn=Volkova\, Anna` | A comma inside a name, which must stay escaped in the tree, the table and both export formats |
| `cn=Анна Волкова` | Non-ASCII in the name, the title and the description — LDIF must base64-encode it |
| `cn=Semi Colon` | Semicolons inside a value, which is what the CSV export joins multiple values on |
| `cn=Long Description` | A 900-character value |
| `cn=No Mail` | An entry missing an attribute the table has a column for |

Regenerate with `python3 dev/directory/generate.py > dev/directory/seed.ldif`.
The generator uses a fixed seed, so the directory is the same every time.
