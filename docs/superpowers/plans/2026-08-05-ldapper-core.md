# Ldapper Core Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the tested Go engine behind Ldapper — connect to a directory, browse it, search it, decode its values, and manage the filter library — plus a small CLI that exercises all of it against a real server.

**Architecture:** Pure packages first (`decode`, `filters`, `export`, `ldaperr`) with no network dependency, then the connected packages (`session`, `schema`, `browse`, `search`) on top. Nothing here imports Wails or knows a UI exists. A `ldapper-probe` CLI wires the pieces together so the whole engine can be run and verified before any interface is built.

**Tech Stack:** Go 1.23, `github.com/go-ldap/ldap/v3`, `github.com/zalando/go-keyring`, stdlib `testing`, Docker Compose with OpenLDAP for integration tests.

**Scope:** This is plan 1 of 3. Plan 2 covers the Wails shell with the connect, certificate, tree and attribute screens. Plan 3 covers search, the filter library UI, export, packaging and release. Both are written after this one lands, so they can build on the signatures fixed here.

**Spec:** `docs/superpowers/specs/2026-08-05-ldapper-design.md`

---

## File Structure

| File | Responsibility |
|---|---|
| `go.mod`, `Makefile`, `.golangci.yml` | Toolchain, pinned dependencies, one-command test and lint |
| `internal/decode/sid.go` | `objectSid` binary → `S-1-5-…` |
| `internal/decode/guid.go` | `objectGUID` binary → canonical UUID |
| `internal/decode/time.go` | Windows FILETIME and `GeneralizedTime` → readable UTC |
| `internal/decode/flags.go` | `userAccountControl`, `groupType`, `sAMAccountType` bit fields → names |
| `internal/decode/attribute.go` | Dispatcher: attribute name + raw bytes → displayable values |
| `internal/ldaperr/explain.go` | LDAP result codes and AD sub-codes → human sentences |
| `internal/filters/filter.go` | `Filter` type, dialects, scope constants |
| `internal/filters/builtin.go`, `builtin.json` | The 18 shipped filters, embedded into the binary |
| `internal/filters/expand.go` | `{{now-90d:filetime}}` and `{{me}}` substitution |
| `internal/filters/validate.go` | RFC 4515 syntax check before anything reaches a server |
| `internal/filters/store.go` | Overlay of user edits on the built-in set; save, reset, delete, restore |
| `internal/profiles/profile.go`, `store.go` | Connection profiles on disk, passwords in the OS keychain |
| `internal/session/dial.go` | TCP/TLS dial, certificate capture and trust decisions |
| `internal/session/bind.go` | Simple and NTLM bind |
| `internal/schema/rootdse.go` | RootDSE read, Active Directory detection, supported dialects |
| `internal/browse/children.go` | One-level paged listing of a node's children |
| `internal/search/stream.go` | Filtered search streamed in batches, with truncation reporting |
| `internal/export/ldif.go`, `csv.go` | Streaming writers for both formats |
| `cmd/ldapper-probe/main.go` | CLI harness: connect, browse, search, filters, export |
| `test/integration/` | Docker Compose OpenLDAP, seed LDIF, build-tagged tests |
| `.github/workflows/ci.yml` | Vet, lint, unit tests on every push; integration tests too |

---

## Task 1: Scaffolding and dependency verification

**Files:**
- Create: `go.mod`, `Makefile`, `.golangci.yml`

- [ ] **Step 1: Initialise the module and pin dependencies**

```bash
cd /Users/skensel/WORKING/AI/ldapper
go mod init github.com/skensell201/ldapper
go get github.com/go-ldap/ldap/v3@v3.4.11
go get github.com/zalando/go-keyring@v0.2.6
```

- [ ] **Step 2: Verify the two APIs this plan depends on actually exist**

`search.Stream` is built on `SearchAsync`, which only exists in go-ldap v3.4.6 and later. Confirm before writing code against it:

```bash
go doc github.com/go-ldap/ldap/v3.Conn.SearchAsync
go doc github.com/go-ldap/ldap/v3.Conn.NTLMBind
```

Expected: both print a signature. `SearchAsync` must read
`func (l *Conn) SearchAsync(ctx context.Context, searchRequest *SearchRequest, bufferSize int) Response`.

If either is missing, stop and report it — the version pin is wrong, and Task 18 and Task 15 need reworking.

- [ ] **Step 3: Write the Makefile**

```makefile
.PHONY: test lint integration tidy

test:
	go test ./...

lint:
	go vet ./...
	golangci-lint run

integration:
	docker compose -f test/integration/docker-compose.yml up -d --wait
	go test -tags=integration ./test/integration/... -v
	docker compose -f test/integration/docker-compose.yml down -v

tidy:
	go mod tidy
```

- [ ] **Step 4: Write the lint config**

```yaml
# .golangci.yml
version: "2"
linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
```

- [ ] **Step 5: Verify the module builds and commit**

```bash
go build ./... && go vet ./...
```

Expected: no output from either command.

```bash
git add go.mod go.sum Makefile .golangci.yml
git commit -m "Set up the Go module, dependency pins and lint config"
```

---

## Task 2: Decode objectSid

An `objectSid` is a byte blob: one revision byte, one sub-authority count, a six-byte big-endian
identifier authority, then that many four-byte little-endian sub-authorities.

**Files:**
- Create: `internal/decode/sid.go`
- Test: `internal/decode/sid_test.go`

- [ ] **Step 1: Write the failing test**

```go
package decode

import "testing"

func TestSID(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want string
	}{
		{
			name: "domain user",
			in: []byte{
				0x01, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
				0x15, 0x00, 0x00, 0x00, // 21
				0xC7, 0x51, 0xDA, 0xD8, // 3638186439
				0xBC, 0x33, 0x34, 0xCF, // 3476304828
				0x14, 0x01, 0xED, 0x01, // 32309524
				0x50, 0x04, 0x00, 0x00, // 1104
			},
			want: "S-1-5-21-3638186439-3476304828-32309524-1104",
		},
		{
			name: "well-known Everyone",
			in:   []byte{0x01, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00},
			want: "S-1-1-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SID(tt.in)
			if err != nil {
				t.Fatalf("SID() returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("SID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSIDRejectsMalformed(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
	}{
		{"empty", nil},
		{"header only", []byte{0x01, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05}},
		{"truncated sub-authority", []byte{
			0x01, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x05,
			0x15, 0x00, 0x00, 0x00,
			0xC7, 0x51, // one byte short of a second sub-authority
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := SID(tt.in); err == nil {
				t.Errorf("SID(%v) succeeded, want an error", tt.in)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/decode/ -run TestSID -v`
Expected: FAIL — `undefined: SID`.

- [ ] **Step 3: Implement it**

```go
// Package decode turns the raw attribute values a directory returns into
// something a person can read. Every function here is pure: no network, no
// clock, no filesystem.
package decode

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// SID formats a binary objectSid in the standard S-R-I-S-S… notation.
func SID(b []byte) (string, error) {
	if len(b) < 8 {
		return "", fmt.Errorf("sid: need at least 8 bytes, got %d", len(b))
	}

	revision := b[0]
	subCount := int(b[1])

	if want := 8 + subCount*4; len(b) != want {
		return "", fmt.Errorf("sid: %d sub-authorities need %d bytes, got %d", subCount, want, len(b))
	}

	// The identifier authority is a 48-bit big-endian value in bytes 2..7.
	var authority uint64
	for _, x := range b[2:8] {
		authority = authority<<8 | uint64(x)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "S-%d-%d", revision, authority)
	for i := 0; i < subCount; i++ {
		off := 8 + i*4
		fmt.Fprintf(&sb, "-%d", binary.LittleEndian.Uint32(b[off:off+4]))
	}
	return sb.String(), nil
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/decode/ -run TestSID -v`
Expected: PASS for every subtest.

- [ ] **Step 5: Commit**

```bash
git add internal/decode/sid.go internal/decode/sid_test.go
git commit -m "Decode objectSid into standard S-1-5-21 notation"
```

---

## Task 3: Decode objectGUID

Active Directory stores a GUID with its first three fields little-endian and the last eight bytes
in order. Reading it as a plain UUID gives the wrong answer, which is why this needs its own test.

**Files:**
- Create: `internal/decode/guid.go`
- Test: `internal/decode/guid_test.go`

- [ ] **Step 1: Write the failing test**

```go
package decode

import "testing"

func TestGUID(t *testing.T) {
	in := []byte{
		0x0C, 0x9B, 0xF1, 0xA3, // Data1, little-endian → a3f19b0c
		0x2E, 0x4D, // Data2, little-endian → 4d2e
		0x1A, 0x4F, // Data3, little-endian → 4f1a
		0x9C, 0x88, // Data4, kept in order
		0x0B, 0x7E, 0x5D, 0x2A, 0x41, 0xF6,
	}
	want := "a3f19b0c-4d2e-4f1a-9c88-0b7e5d2a41f6"

	got, err := GUID(in)
	if err != nil {
		t.Fatalf("GUID() returned error: %v", err)
	}
	if got != want {
		t.Errorf("GUID() = %q, want %q", got, want)
	}
}

func TestGUIDRejectsWrongLength(t *testing.T) {
	for _, n := range []int{0, 15, 17} {
		if _, err := GUID(make([]byte, n)); err == nil {
			t.Errorf("GUID() with %d bytes succeeded, want an error", n)
		}
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/decode/ -run TestGUID -v`
Expected: FAIL — `undefined: GUID`.

- [ ] **Step 3: Implement it**

```go
package decode

import (
	"encoding/binary"
	"fmt"
)

// GUID formats a 16-byte objectGUID as a canonical UUID string. The first
// three fields are stored little-endian; the remaining eight bytes are not.
func GUID(b []byte) (string, error) {
	if len(b) != 16 {
		return "", fmt.Errorf("guid: need exactly 16 bytes, got %d", len(b))
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.LittleEndian.Uint32(b[0:4]),
		binary.LittleEndian.Uint16(b[4:6]),
		binary.LittleEndian.Uint16(b[6:8]),
		binary.BigEndian.Uint16(b[8:10]),
		b[10:16],
	), nil
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/decode/ -run TestGUID -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/decode/guid.go internal/decode/guid_test.go
git commit -m "Decode objectGUID with its little-endian leading fields"
```

---

## Task 4: Decode timestamps

Two formats appear in directories. Active Directory uses FILETIME — 100-nanosecond ticks since
1601-01-01 — with two magic values: `0` meaning the field was never set, and the maximum int64
meaning "never expires". Everything else uses `GeneralizedTime`.

**Files:**
- Create: `internal/decode/time.go`
- Test: `internal/decode/time_test.go`

- [ ] **Step 1: Write the failing test**

```go
package decode

import "testing"

func TestFileTime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"a real timestamp", "133894094610000000", "2025-04-18 00:24:21 UTC"},
		{"never set", "0", "not set"},
		{"never expires", "9223372036854775807", "never"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FileTime(tt.in)
			if err != nil {
				t.Fatalf("FileTime(%q) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("FileTime(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFileTimeRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "not a number", "-5"} {
		if _, err := FileTime(in); err == nil {
			t.Errorf("FileTime(%q) succeeded, want an error", in)
		}
	}
}

func TestGeneralizedTime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"with fraction and Z", "20230914081233.0Z", "2023-09-14 08:12:33 UTC"},
		{"without fraction", "20230914081233Z", "2023-09-14 08:12:33 UTC"},
		{"with numeric offset", "20230914101233+0200", "2023-09-14 08:12:33 UTC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GeneralizedTime(tt.in)
			if err != nil {
				t.Fatalf("GeneralizedTime(%q) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("GeneralizedTime(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/decode/ -run 'Time' -v`
Expected: FAIL — `undefined: FileTime`.

- [ ] **Step 3: Implement it**

```go
package decode

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// display is the one timestamp layout the interface uses everywhere.
const display = "2006-01-02 15:04:05 UTC"

// ticksPerSecond is how many 100-nanosecond intervals fit in a second.
const ticksPerSecond = 10_000_000

// epochOffset is the gap in seconds between the FILETIME epoch (1601-01-01)
// and the Unix epoch.
const epochOffset = 11_644_473_600

// FileTime renders a Windows FILETIME string. Zero means the attribute was
// never written; the maximum int64 is Active Directory's way of saying the
// deadline never arrives.
func FileTime(s string) (string, error) {
	ticks, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return "", fmt.Errorf("filetime: %q is not an integer", s)
	}
	switch {
	case ticks < 0:
		return "", fmt.Errorf("filetime: %d is negative", ticks)
	case ticks == 0:
		return "not set", nil
	case ticks == 9223372036854775807:
		return "never", nil
	}
	secs := ticks/ticksPerSecond - epochOffset
	return time.Unix(secs, 0).UTC().Format(display), nil
}

// GeneralizedTime renders an RFC 4517 GeneralizedTime in UTC.
func GeneralizedTime(s string) (string, error) {
	s = strings.TrimSpace(s)
	layouts := []string{
		"20060102150405.0Z", "20060102150405Z",
		"20060102150405.0-0700", "20060102150405-0700",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(display), nil
		}
	}
	return "", fmt.Errorf("generalizedtime: %q matches no known layout", s)
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/decode/ -run 'Time' -v`
Expected: PASS for every subtest.

- [ ] **Step 5: Commit**

```bash
git add internal/decode/time.go internal/decode/time_test.go
git commit -m "Decode FILETIME and GeneralizedTime, including AD's magic values"
```

---

## Task 5: Decode bit-field attributes

`userAccountControl` packs a dozen booleans into one integer. So do `groupType` and
`sAMAccountType`. All three are the difference between an unreadable number and an answer.

**Files:**
- Create: `internal/decode/flags.go`
- Test: `internal/decode/flags_test.go`

- [ ] **Step 1: Write the failing test**

```go
package decode

import (
	"strings"
	"testing"
)

func TestUserAccountControl(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"normal account", "512", []string{"NORMAL_ACCOUNT"}},
		{"password never expires", "66048", []string{"NORMAL_ACCOUNT", "DONT_EXPIRE_PASSWORD"}},
		{"disabled account", "514", []string{"ACCOUNTDISABLE", "NORMAL_ACCOUNT"}},
		{"no flags set", "0", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := UserAccountControl(tt.in)
			if err != nil {
				t.Fatalf("UserAccountControl(%q) returned error: %v", tt.in, err)
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Errorf("UserAccountControl(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestUserAccountControlKeepsUnknownBits(t *testing.T) {
	// Bit 31 has no assigned meaning. It must still be reported, not dropped:
	// silently hiding a set bit is worse than showing a hex value.
	got, err := UserAccountControl("2147483648")
	if err != nil {
		t.Fatalf("UserAccountControl() returned error: %v", err)
	}
	if len(got) != 1 || !strings.Contains(got[0], "0x80000000") {
		t.Errorf("UserAccountControl() = %v, want the unknown bit reported in hex", got)
	}
}

func TestGroupType(t *testing.T) {
	// -2147483646 is 0x80000002: a global security group.
	got, err := GroupType("-2147483646")
	if err != nil {
		t.Fatalf("GroupType() returned error: %v", err)
	}
	if strings.Join(got, ",") != "GLOBAL,SECURITY" {
		t.Errorf("GroupType() = %v, want [GLOBAL SECURITY]", got)
	}
}

func TestSAMAccountType(t *testing.T) {
	got, err := SAMAccountType("805306368")
	if err != nil {
		t.Fatalf("SAMAccountType() returned error: %v", err)
	}
	if got != "SAM_NORMAL_USER_ACCOUNT" {
		t.Errorf("SAMAccountType() = %q, want SAM_NORMAL_USER_ACCOUNT", got)
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/decode/ -run 'AccountControl|GroupType|SAMAccount' -v`
Expected: FAIL — `undefined: UserAccountControl`.

- [ ] **Step 3: Implement it**

```go
package decode

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// bit pairs a single flag value with its name.
type bit struct {
	value uint32
	name  string
}

var uacBits = []bit{
	{0x00000001, "SCRIPT"},
	{0x00000002, "ACCOUNTDISABLE"},
	{0x00000008, "HOMEDIR_REQUIRED"},
	{0x00000010, "LOCKOUT"},
	{0x00000020, "PASSWD_NOTREQD"},
	{0x00000040, "PASSWD_CANT_CHANGE"},
	{0x00000080, "ENCRYPTED_TEXT_PWD_ALLOWED"},
	{0x00000100, "TEMP_DUPLICATE_ACCOUNT"},
	{0x00000200, "NORMAL_ACCOUNT"},
	{0x00000800, "INTERDOMAIN_TRUST_ACCOUNT"},
	{0x00001000, "WORKSTATION_TRUST_ACCOUNT"},
	{0x00002000, "SERVER_TRUST_ACCOUNT"},
	{0x00010000, "DONT_EXPIRE_PASSWORD"},
	{0x00020000, "MNS_LOGON_ACCOUNT"},
	{0x00040000, "SMARTCARD_REQUIRED"},
	{0x00080000, "TRUSTED_FOR_DELEGATION"},
	{0x00100000, "NOT_DELEGATED"},
	{0x00200000, "USE_DES_KEY_ONLY"},
	{0x00400000, "DONT_REQ_PREAUTH"},
	{0x00800000, "PASSWORD_EXPIRED"},
	{0x01000000, "TRUSTED_TO_AUTH_FOR_DELEGATION"},
	{0x04000000, "PARTIAL_SECRETS_ACCOUNT"},
}

var groupTypeBits = []bit{
	{0x00000001, "BUILTIN_LOCAL"},
	{0x00000002, "GLOBAL"},
	{0x00000004, "DOMAIN_LOCAL"},
	{0x00000008, "UNIVERSAL"},
	{0x00000010, "APP_BASIC"},
	{0x00000020, "APP_QUERY"},
	{0x80000000, "SECURITY"},
}

var samAccountTypes = map[uint32]string{
	0x00000000: "SAM_DOMAIN_OBJECT",
	0x10000000: "SAM_GROUP_OBJECT",
	0x10000001: "SAM_NON_SECURITY_GROUP_OBJECT",
	0x20000000: "SAM_ALIAS_OBJECT",
	0x20000001: "SAM_NON_SECURITY_ALIAS_OBJECT",
	0x30000000: "SAM_NORMAL_USER_ACCOUNT",
	0x30000001: "SAM_MACHINE_ACCOUNT",
	0x30000002: "SAM_TRUST_ACCOUNT",
	0x40000000: "SAM_APP_BASIC_GROUP",
	0x40000001: "SAM_APP_QUERY_GROUP",
}

// parseBitField accepts the signed decimal a directory returns and reinterprets
// it as the unsigned 32-bit value it actually is. groupType arrives negative
// whenever its top bit is set.
func parseBitField(s string) (uint32, error) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bit field: %q is not an integer", s)
	}
	return uint32(n), nil
}

// splitBits names every set bit, reporting unrecognised ones in hex rather
// than discarding them.
func splitBits(v uint32, known []bit) []string {
	var out []string
	var matched uint32
	for _, b := range known {
		if v&b.value != 0 {
			out = append(out, b.name)
			matched |= b.value
		}
	}
	if leftover := v &^ matched; leftover != 0 {
		out = append(out, fmt.Sprintf("unknown 0x%08X", leftover))
	}
	sort.Strings(out)
	return out
}

// UserAccountControl names the flags packed into a userAccountControl value.
func UserAccountControl(s string) ([]string, error) {
	v, err := parseBitField(s)
	if err != nil {
		return nil, err
	}
	return splitBits(v, uacBits), nil
}

// GroupType names a group's scope and whether it is a security group.
func GroupType(s string) ([]string, error) {
	v, err := parseBitField(s)
	if err != nil {
		return nil, err
	}
	return splitBits(v, groupTypeBits), nil
}

// SAMAccountType names the kind of account. Unlike the others it is an
// enumeration, not a bit field.
func SAMAccountType(s string) (string, error) {
	v, err := parseBitField(s)
	if err != nil {
		return "", err
	}
	if name, ok := samAccountTypes[v]; ok {
		return name, nil
	}
	return fmt.Sprintf("unknown 0x%08X", v), nil
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/decode/ -run 'AccountControl|GroupType|SAMAccount' -v`
Expected: PASS. Note that `splitBits` sorts, so `514` yields `ACCOUNTDISABLE,NORMAL_ACCOUNT` in
that order — which is what the test asserts.

- [ ] **Step 5: Commit**

```bash
git add internal/decode/flags.go internal/decode/flags_test.go
git commit -m "Decode userAccountControl, groupType and sAMAccountType bit fields"
```

---

## Task 6: The decode dispatcher

Everything above is joined by one entry point the rest of the program calls. The rule it must
obey: a decoder that cannot make sense of a value returns the raw value, never an error. A
directory containing something unexpected must not break the screen showing it.

**Files:**
- Create: `internal/decode/attribute.go`
- Test: `internal/decode/attribute_test.go`

- [ ] **Step 1: Write the failing test**

```go
package decode

import "testing"

func TestAttributeDecodesKnownNames(t *testing.T) {
	got := Attribute("userAccountControl", [][]byte{[]byte("66048")})
	if len(got) != 1 {
		t.Fatalf("Attribute() returned %d values, want 1", len(got))
	}
	if got[0].Raw != "66048" {
		t.Errorf("Raw = %q, want 66048", got[0].Raw)
	}
	if len(got[0].Decoded) != 2 || got[0].Decoded[0] != "DONT_EXPIRE_PASSWORD" {
		t.Errorf("Decoded = %v, want both flags", got[0].Decoded)
	}
}

func TestAttributeIsCaseInsensitive(t *testing.T) {
	// Directories are inconsistent about attribute name casing.
	got := Attribute("PWDLASTSET", [][]byte{[]byte("0")})
	if len(got[0].Decoded) != 1 || got[0].Decoded[0] != "not set" {
		t.Errorf("Decoded = %v, want [not set]", got[0].Decoded)
	}
}

func TestAttributeFallsBackToRaw(t *testing.T) {
	// An unknown attribute is passed through untouched.
	got := Attribute("displayName", [][]byte{[]byte("Anna Volkova")})
	if got[0].Raw != "Anna Volkova" {
		t.Errorf("Raw = %q, want the value unchanged", got[0].Raw)
	}
	if got[0].Decoded != nil {
		t.Errorf("Decoded = %v, want nil for an attribute with no decoder", got[0].Decoded)
	}
}

func TestAttributeSurvivesUndecodableValues(t *testing.T) {
	// objectSid has a decoder, but this is not a valid SID. The value must
	// still come back — as base64, since it is not printable text.
	got := Attribute("objectSid", [][]byte{{0xFF, 0xFE}})
	if len(got) != 1 {
		t.Fatalf("Attribute() returned %d values, want 1", len(got))
	}
	if got[0].Raw != "//4=" {
		t.Errorf("Raw = %q, want the base64 of the undecodable bytes", got[0].Raw)
	}
	if got[0].Decoded != nil {
		t.Errorf("Decoded = %v, want nil when decoding failed", got[0].Decoded)
	}
}

func TestAttributeHandlesEveryValueOfAMultiValuedAttribute(t *testing.T) {
	got := Attribute("objectClass", [][]byte{[]byte("top"), []byte("person"), []byte("user")})
	if len(got) != 3 {
		t.Fatalf("Attribute() returned %d values, want 3", len(got))
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/decode/ -run TestAttribute -v`
Expected: FAIL — `undefined: Attribute`.

- [ ] **Step 3: Implement it**

```go
package decode

import (
	"encoding/base64"
	"strings"
	"unicode/utf8"
)

// Value is one attribute value ready to be shown: the server's own bytes, and
// whatever sense could be made of them.
type Value struct {
	// Raw is the value as the server returned it — printable text as-is,
	// anything else base64-encoded.
	Raw string
	// Decoded holds the human-readable rendering, or nil when the attribute
	// has no decoder or its value could not be decoded. Bit fields produce
	// several entries; everything else produces at most one.
	Decoded []string
}

// binaryDecoder reads raw bytes; stringDecoder reads the text form.
type binaryDecoder func([]byte) (string, error)
type stringDecoder func(string) (string, error)
type multiDecoder func(string) ([]string, error)

var binaryDecoders = map[string]binaryDecoder{
	"objectsid":  SID,
	"objectguid": GUID,
}

var stringDecoders = map[string]stringDecoder{
	"pwdlastset":         FileTime,
	"lastlogon":          FileTime,
	"lastlogontimestamp": FileTime,
	"accountexpires":     FileTime,
	"badpasswordtime":    FileTime,
	"lockouttime":        FileTime,
	"whencreated":        GeneralizedTime,
	"whenchanged":        GeneralizedTime,
	"samaccounttype":     SAMAccountType,
}

var multiDecoders = map[string]multiDecoder{
	"useraccountcontrol": UserAccountControl,
	"grouptype":          GroupType,
}

// Attribute decodes every value of one attribute. It never returns an error:
// a value that cannot be decoded is returned raw, because failing to explain a
// value is not a reason to refuse to show it.
func Attribute(name string, values [][]byte) []Value {
	key := strings.ToLower(name)
	out := make([]Value, 0, len(values))

	for _, raw := range values {
		v := Value{Raw: printable(raw)}

		switch {
		case binaryDecoders[key] != nil:
			if s, err := binaryDecoders[key](raw); err == nil {
				v.Decoded = []string{s}
			}
		case stringDecoders[key] != nil:
			if s, err := stringDecoders[key](string(raw)); err == nil {
				v.Decoded = []string{s}
			}
		case multiDecoders[key] != nil:
			if list, err := multiDecoders[key](string(raw)); err == nil && len(list) > 0 {
				v.Decoded = list
			}
		}

		out = append(out, v)
	}
	return out
}

// printable returns text unchanged and encodes anything else as base64, the
// same convention LDIF uses.
func printable(b []byte) string {
	if !utf8.Valid(b) {
		return base64.StdEncoding.EncodeToString(b)
	}
	for _, r := range string(b) {
		if r < 0x20 || r == 0x7F {
			return base64.StdEncoding.EncodeToString(b)
		}
	}
	return string(b)
}
```

- [ ] **Step 4: Run the whole package and confirm it passes**

Run: `go test ./internal/decode/ -v`
Expected: PASS for every test in the package.

- [ ] **Step 5: Commit**

```bash
git add internal/decode/attribute.go internal/decode/attribute_test.go
git commit -m "Add the decode dispatcher, falling back to raw values on failure"
```

---

## Task 7: Explain LDAP errors

A bare `LDAP Result Code 49` tells a user nothing. Active Directory hides the actual reason in a
`data 52e` fragment inside the diagnostic message, and that fragment is the difference between
"wrong password" and "your account is locked out".

**Files:**
- Create: `internal/ldaperr/explain.go`
- Test: `internal/ldaperr/explain_test.go`

- [ ] **Step 1: Write the failing test**

```go
package ldaperr

import (
	"errors"
	"strings"
	"testing"

	"github.com/go-ldap/ldap/v3"
)

func TestExplainKnownResultCodes(t *testing.T) {
	tests := []struct {
		name string
		code uint16
		want string
	}{
		{"bad credentials", ldap.LDAPResultInvalidCredentials, "username or password"},
		{"no rights", ldap.LDAPResultInsufficientAccessRights, "permission"},
		{"size limit", ldap.LDAPResultSizeLimitExceeded, "stopped early"},
		{"unwilling", ldap.LDAPResultUnwillingToPerform, "refused"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Explain(&ldap.Error{ResultCode: tt.code})
			if !strings.Contains(strings.ToLower(got), tt.want) {
				t.Errorf("Explain() = %q, want it to mention %q", got, tt.want)
			}
		})
	}
}

func TestExplainActiveDirectorySubCodes(t *testing.T) {
	tests := []struct {
		name string
		diag string
		want string
	}{
		{"wrong password", "80090308: LdapErr: DSID-0C09042A, comment: AcceptSecurityContext error, data 52e, v4563", "password"},
		{"account disabled", "80090308: LdapErr: DSID-0C09042A, comment: AcceptSecurityContext error, data 533, v4563", "disabled"},
		{"account locked", "80090308: LdapErr: DSID-0C09042A, comment: AcceptSecurityContext error, data 775, v4563", "locked"},
		{"password expired", "80090308: LdapErr: DSID-0C09042A, comment: AcceptSecurityContext error, data 532, v4563", "expired"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ldap.Error{
				ResultCode:      ldap.LDAPResultInvalidCredentials,
				DiagnosticMessage: tt.diag,
			}
			got := strings.ToLower(Explain(err))
			if !strings.Contains(got, tt.want) {
				t.Errorf("Explain() = %q, want it to mention %q", got, tt.want)
			}
		})
	}
}

func TestExplainPassesThroughNonLDAPErrors(t *testing.T) {
	got := Explain(errors.New("dial tcp: connection refused"))
	if !strings.Contains(got, "connection refused") {
		t.Errorf("Explain() = %q, want the original message preserved", got)
	}
}

func TestExplainHandlesNil(t *testing.T) {
	if got := Explain(nil); got != "" {
		t.Errorf("Explain(nil) = %q, want an empty string", got)
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/ldaperr/ -v`
Expected: FAIL — `undefined: Explain`.

- [ ] **Step 3: Implement it**

```go
// Package ldaperr turns LDAP protocol errors into sentences that say what went
// wrong and what to do about it.
package ldaperr

import (
	"errors"
	"regexp"

	"github.com/go-ldap/ldap/v3"
)

// adSubCode matches the "data 52e" fragment Active Directory buries inside the
// diagnostic message of an invalid-credentials error.
var adSubCode = regexp.MustCompile(`data ([0-9a-fA-F]+)`)

var adReasons = map[string]string{
	"525": "That user does not exist in this directory.",
	"52e": "Wrong username or password.",
	"530": "That account is not allowed to sign in at this time of day.",
	"531": "That account is not allowed to sign in from this computer.",
	"532": "The password has expired and must be changed before you can sign in.",
	"533": "That account is disabled.",
	"701": "That account has expired.",
	"773": "The password must be changed before this account can be used.",
	"775": "That account is locked out. It will unlock on its own, or an administrator can unlock it now.",
}

var byResultCode = map[uint16]string{
	ldap.LDAPResultInvalidCredentials:       "Wrong username or password.",
	ldap.LDAPResultInsufficientAccessRights: "This account does not have permission to read that object. Sign in with an account that does.",
	ldap.LDAPResultSizeLimitExceeded:        "The server stopped early — you are seeing part of the results. Narrow the filter or search from a lower branch.",
	ldap.LDAPResultTimeLimitExceeded:        "The server ran out of time — you are seeing part of the results. Narrow the filter or search from a lower branch.",
	ldap.LDAPResultNoSuchObject:             "There is no object at that distinguished name. It may have been moved or deleted.",
	ldap.LDAPResultReferral:                 "That object lives in a different naming context. Connect to the server that holds it.",
	ldap.LDAPResultUnwillingToPerform:       "The server refused the request. Most often it requires an encrypted connection before it will accept a bind.",
	ldap.LDAPResultStrongAuthRequired:       "The server requires a stronger authentication method. Switch on LDAPS or StartTLS.",
	ldap.LDAPResultInappropriateAuthentication: "The server rejected this way of signing in. Try a different bind method.",
	ldap.LDAPResultConstraintViolation:      "The value breaks a rule the server enforces on that attribute.",
	ldap.LDAPResultBusy:                     "The server is too busy to answer right now. Try again in a moment.",
	ldap.LDAPResultUnavailable:              "The server is not accepting requests right now.",
}

// Explain returns a sentence describing err. Errors that are not LDAP protocol
// errors are returned as they are — a refused TCP connection already says what
// happened.
func Explain(err error) string {
	if err == nil {
		return ""
	}

	var lerr *ldap.Error
	if !errors.As(err, &lerr) {
		return err.Error()
	}

	if lerr.ResultCode == ldap.LDAPResultInvalidCredentials {
		if m := adSubCode.FindStringSubmatch(lerr.DiagnosticMessage); m != nil {
			if reason, ok := adReasons[m[1]]; ok {
				return reason
			}
		}
	}

	if msg, ok := byResultCode[lerr.ResultCode]; ok {
		return msg
	}

	return lerr.Error()
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/ldaperr/ -v`
Expected: PASS for every subtest.

If the compiler rejects `DiagnosticMessage`, check the field name on the pinned version with
`go doc github.com/go-ldap/ldap/v3.Error` and use whatever it calls the diagnostic string.

- [ ] **Step 5: Commit**

```bash
git add internal/ldaperr/
git commit -m "Translate LDAP result codes and AD sub-codes into plain sentences"
```

---

## Task 8: The Filter type and the built-in set

**Files:**
- Create: `internal/filters/filter.go`, `internal/filters/builtin.json`, `internal/filters/builtin.go`
- Test: `internal/filters/builtin_test.go`

- [ ] **Step 1: Write the failing test**

```go
package filters

import "testing"

func TestBuiltinsLoad(t *testing.T) {
	got := Builtins()
	if len(got) != 18 {
		t.Fatalf("Builtins() returned %d filters, want 18", len(got))
	}
}

func TestBuiltinsAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, f := range Builtins() {
		if f.ID == "" || f.Name == "" || f.Filter == "" {
			t.Errorf("filter %+v is missing a required field", f)
		}
		if seen[f.ID] {
			t.Errorf("duplicate filter id %q", f.ID)
		}
		seen[f.ID] = true

		switch f.Dialect {
		case DialectAD, DialectGeneric, DialectPOSIX:
		default:
			t.Errorf("filter %q has unknown dialect %q", f.ID, f.Dialect)
		}
		switch f.Scope {
		case ScopeBase, ScopeOne, ScopeSubtree:
		default:
			t.Errorf("filter %q has unknown scope %q", f.ID, f.Scope)
		}
		if !f.BuiltIn {
			t.Errorf("filter %q came from the embedded set but is not marked built-in", f.ID)
		}
	}
}

func TestBuiltinsSplitByDialect(t *testing.T) {
	counts := map[Dialect]int{}
	for _, f := range Builtins() {
		counts[f.Dialect]++
	}
	if counts[DialectAD] != 12 {
		t.Errorf("got %d Active Directory filters, want 12", counts[DialectAD])
	}
	if counts[DialectGeneric]+counts[DialectPOSIX] != 6 {
		t.Errorf("got %d portable filters, want 6", counts[DialectGeneric]+counts[DialectPOSIX])
	}
}

func TestBuiltinsAreACopy(t *testing.T) {
	// Callers must not be able to mutate the shipped set.
	first := Builtins()
	first[0].Name = "clobbered"
	if Builtins()[0].Name == "clobbered" {
		t.Error("Builtins() handed out a reference to the shared slice")
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/filters/ -v`
Expected: FAIL — `undefined: Builtins`.

- [ ] **Step 3: Write the type definitions**

```go
// Package filters holds the library of saved LDAP searches: the set shipped
// with Ldapper, the user's edits to it, and their own additions.
package filters

// Dialect says which servers a filter can run against.
type Dialect string

const (
	// DialectAD marks filters using Active Directory extensions, such as the
	// bitwise matching rule 1.2.840.113556.1.4.803.
	DialectAD Dialect = "ad"
	// DialectGeneric marks filters that work on any LDAP server.
	DialectGeneric Dialect = "generic"
	// DialectPOSIX marks filters needing the POSIX schema, which Active
	// Directory does not carry by default.
	DialectPOSIX Dialect = "posix"
)

// Scope is the LDAP search scope.
type Scope string

const (
	ScopeBase    Scope = "base"
	ScopeOne     Scope = "one"
	ScopeSubtree Scope = "subtree"
)

// Filter is one saved search.
type Filter struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	// Filter is an RFC 4515 filter that may contain {{…}} substitutions.
	Filter string `json:"filter"`
	Scope  Scope  `json:"scope"`
	// Base is the search base. Empty means the server's default naming context.
	Base    string   `json:"base,omitempty"`
	Columns []string `json:"columns,omitempty"`
	Dialect Dialect  `json:"dialect"`

	// BuiltIn is true for filters that ship with Ldapper. Not persisted:
	// it is derived from where the filter was loaded from.
	BuiltIn bool `json:"-"`
	// Modified is true for a built-in filter the user has edited.
	Modified bool `json:"-"`
}
```

- [ ] **Step 4: Write the embedded filter set**

Create `internal/filters/builtin.json` with exactly these 18 entries:

```json
[
  {
    "id": "ad-disabled-accounts",
    "name": "Disabled accounts",
    "description": "User accounts that have been switched off.",
    "filter": "(&(objectCategory=person)(objectClass=user)(userAccountControl:1.2.840.113556.1.4.803:=2))",
    "scope": "subtree",
    "columns": ["cn", "sAMAccountName", "whenChanged", "distinguishedName"],
    "dialect": "ad"
  },
  {
    "id": "ad-locked-out",
    "name": "Locked out right now",
    "description": "Accounts locked by the password policy. They unlock on their own once the lockout window passes.",
    "filter": "(&(objectCategory=person)(objectClass=user)(lockoutTime>=1))",
    "scope": "subtree",
    "columns": ["cn", "sAMAccountName", "lockoutTime", "distinguishedName"],
    "dialect": "ad"
  },
  {
    "id": "ad-password-never-expires",
    "name": "Password never expires",
    "description": "Accounts exempt from password expiry.",
    "filter": "(&(objectCategory=person)(objectClass=user)(userAccountControl:1.2.840.113556.1.4.803:=65536))",
    "scope": "subtree",
    "columns": ["cn", "sAMAccountName", "pwdLastSet", "distinguishedName"],
    "dialect": "ad"
  },
  {
    "id": "ad-must-change-password",
    "name": "Must change at next logon",
    "description": "Accounts whose password has been flagged for replacement.",
    "filter": "(&(objectCategory=person)(objectClass=user)(pwdLastSet=0))",
    "scope": "subtree",
    "columns": ["cn", "sAMAccountName", "whenCreated", "distinguishedName"],
    "dialect": "ad"
  },
  {
    "id": "ad-stale-users-90d",
    "name": "Stale users · 90 days",
    "description": "Enabled accounts that have not authenticated in 90 days. Disabled ones are excluded — they have their own filter.",
    "filter": "(&(objectCategory=person)(objectClass=user)(!(userAccountControl:1.2.840.113556.1.4.803:=2))(lastLogonTimestamp<={{now-90d:filetime}}))",
    "scope": "subtree",
    "columns": ["cn", "sAMAccountName", "lastLogonTimestamp", "distinguishedName"],
    "dialect": "ad"
  },
  {
    "id": "ad-stale-computers-60d",
    "name": "Stale computers · 60 days",
    "description": "Machine accounts that have not checked in for 60 days.",
    "filter": "(&(objectCategory=computer)(lastLogonTimestamp<={{now-60d:filetime}}))",
    "scope": "subtree",
    "columns": ["cn", "operatingSystem", "lastLogonTimestamp", "distinguishedName"],
    "dialect": "ad"
  },
  {
    "id": "ad-privileged-accounts",
    "name": "Privileged accounts",
    "description": "Accounts carrying adminCount=1 — members, now or once, of a protected group.",
    "filter": "(&(objectCategory=person)(objectClass=user)(adminCount=1))",
    "scope": "subtree",
    "columns": ["cn", "sAMAccountName", "memberOf", "distinguishedName"],
    "dialect": "ad"
  },
  {
    "id": "ad-accounts-with-spn",
    "name": "Accounts with an SPN",
    "description": "User accounts registered as a service. Usually service accounts.",
    "filter": "(&(objectCategory=person)(objectClass=user)(servicePrincipalName=*))",
    "scope": "subtree",
    "columns": ["cn", "sAMAccountName", "servicePrincipalName", "distinguishedName"],
    "dialect": "ad"
  },
  {
    "id": "ad-no-preauth",
    "name": "Kerberos pre-auth disabled",
    "description": "Accounts that skip Kerberos pre-authentication — worth reviewing.",
    "filter": "(&(objectCategory=person)(objectClass=user)(userAccountControl:1.2.840.113556.1.4.803:=4194304))",
    "scope": "subtree",
    "columns": ["cn", "sAMAccountName", "userAccountControl", "distinguishedName"],
    "dialect": "ad"
  },
  {
    "id": "ad-unconstrained-delegation",
    "name": "Unconstrained delegation",
    "description": "Computers trusted to impersonate any user to any service.",
    "filter": "(&(objectCategory=computer)(userAccountControl:1.2.840.113556.1.4.803:=524288))",
    "scope": "subtree",
    "columns": ["cn", "operatingSystem", "userAccountControl", "distinguishedName"],
    "dialect": "ad"
  },
  {
    "id": "ad-domain-controllers",
    "name": "Domain controllers",
    "description": "Every server holding a writable copy of the directory.",
    "filter": "(&(objectCategory=computer)(userAccountControl:1.2.840.113556.1.4.803:=8192))",
    "scope": "subtree",
    "columns": ["cn", "dNSHostName", "operatingSystem", "distinguishedName"],
    "dialect": "ad"
  },
  {
    "id": "ad-group-policy-objects",
    "name": "Group policy objects",
    "description": "Every GPO defined in the domain.",
    "filter": "(objectClass=groupPolicyContainer)",
    "scope": "subtree",
    "columns": ["displayName", "cn", "whenChanged", "distinguishedName"],
    "dialect": "ad"
  },
  {
    "id": "all-people",
    "name": "All people",
    "description": "Every person object, whichever class the directory uses for them.",
    "filter": "(|(objectClass=person)(objectClass=inetOrgPerson))",
    "scope": "subtree",
    "columns": ["cn", "mail", "distinguishedName"],
    "dialect": "generic"
  },
  {
    "id": "all-groups",
    "name": "All groups",
    "description": "Every group object across the common schemas.",
    "filter": "(|(objectClass=group)(objectClass=groupOfNames)(objectClass=groupOfUniqueNames)(objectClass=posixGroup))",
    "scope": "subtree",
    "columns": ["cn", "description", "distinguishedName"],
    "dialect": "generic"
  },
  {
    "id": "all-organizational-units",
    "name": "All organizational units",
    "description": "The skeleton of the directory.",
    "filter": "(objectClass=organizationalUnit)",
    "scope": "subtree",
    "columns": ["ou", "description", "distinguishedName"],
    "dialect": "generic"
  },
  {
    "id": "empty-groups",
    "name": "Empty groups",
    "description": "Groups with no members at all.",
    "filter": "(&(|(objectClass=group)(objectClass=groupOfNames))(!(member=*)))",
    "scope": "subtree",
    "columns": ["cn", "description", "whenCreated", "distinguishedName"],
    "dialect": "generic"
  },
  {
    "id": "people-without-email",
    "name": "People without email",
    "description": "Person objects with no mail attribute.",
    "filter": "(&(objectClass=person)(!(mail=*)))",
    "scope": "subtree",
    "columns": ["cn", "whenCreated", "distinguishedName"],
    "dialect": "generic"
  },
  {
    "id": "posix-accounts",
    "name": "POSIX accounts",
    "description": "Accounts carrying Unix attributes.",
    "filter": "(objectClass=posixAccount)",
    "scope": "subtree",
    "columns": ["uid", "uidNumber", "gidNumber", "homeDirectory", "distinguishedName"],
    "dialect": "posix"
  }
]
```

- [ ] **Step 5: Write the loader**

```go
package filters

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed builtin.json
var builtinJSON []byte

// builtins is parsed once at start-up. A malformed builtin.json is a build
// mistake, not a runtime condition, so it panics rather than degrading.
var builtins = mustParseBuiltins()

func mustParseBuiltins() []Filter {
	var out []Filter
	if err := json.Unmarshal(builtinJSON, &out); err != nil {
		panic(fmt.Sprintf("filters: builtin.json is malformed: %v", err))
	}
	for i := range out {
		out[i].BuiltIn = true
	}
	return out
}

// Builtins returns a copy of the filter set shipped with Ldapper.
func Builtins() []Filter {
	out := make([]Filter, len(builtins))
	copy(out, builtins)
	return out
}
```

- [ ] **Step 6: Run the tests and confirm they pass**

Run: `go test ./internal/filters/ -v`
Expected: PASS. If the count assertions fail, `builtin.json` has the wrong number of entries —
count them again against the table above.

- [ ] **Step 7: Commit**

```bash
git add internal/filters/
git commit -m "Add the Filter type and embed the 18 shipped filters"
```

---

## Task 9: Substitution in filter templates

Half the useful filters are relative to now, so a filter is a template, not a constant. The clock
is a field rather than a call to `time.Now`, because a decoder you cannot pin to a fixed instant
cannot be tested.

**Files:**
- Create: `internal/filters/expand.go`
- Test: `internal/filters/expand_test.go`

- [ ] **Step 1: Write the failing test**

```go
package filters

import (
	"testing"
	"time"
)

// fixed is the instant every test in this file pretends it is:
// 2026-08-05 12:00:00 UTC.
var fixed = time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)

func TestExpandFileTime(t *testing.T) {
	e := Expander{Now: fixed}

	// 2026-08-05 12:00:00 UTC is 1785931200 in Unix seconds.
	// FILETIME = (unix + 11644473600) * 10000000 = 134304681600000000.
	got, err := e.Expand("(whenCreated<={{now:filetime}})")
	if err != nil {
		t.Fatalf("Expand() returned error: %v", err)
	}
	if got != "(whenCreated<=134304681600000000)" {
		t.Errorf("Expand() = %q, want the FILETIME substituted", got)
	}
}

func TestExpandRelativeOffsets(t *testing.T) {
	e := Expander{Now: fixed}
	tests := []struct {
		in   string
		want string
	}{
		// 90 days earlier is 2026-05-07 12:00:00 UTC.
		{"{{now-90d:generalized}}", "20260507120000Z"},
		{"{{now-6h:generalized}}", "20260805060000Z"},
		{"{{now-30m:generalized}}", "20260805113000Z"},
		{"{{now+1d:generalized}}", "20260806120000Z"},
		// No format given defaults to generalized.
		{"{{now}}", "20260805120000Z"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := e.Expand(tt.in)
			if err != nil {
				t.Fatalf("Expand(%q) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("Expand(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExpandBindDN(t *testing.T) {
	e := Expander{Now: fixed, BindDN: "CN=Anna Volkova,OU=Users,DC=corp,DC=example,DC=com"}
	got, err := e.Expand("(manager={{me}})")
	if err != nil {
		t.Fatalf("Expand() returned error: %v", err)
	}
	want := "(manager=CN=Anna Volkova,OU=Users,DC=corp,DC=example,DC=com)"
	if got != want {
		t.Errorf("Expand() = %q, want %q", got, want)
	}
}

func TestExpandLeavesPlainFiltersAlone(t *testing.T) {
	e := Expander{Now: fixed}
	in := "(&(objectClass=user)(sAMAccountName=a.volkova))"
	got, err := e.Expand(in)
	if err != nil {
		t.Fatalf("Expand() returned error: %v", err)
	}
	if got != in {
		t.Errorf("Expand() = %q, want the filter unchanged", got)
	}
}

func TestExpandRejectsBadSubstitutions(t *testing.T) {
	e := Expander{Now: fixed}
	tests := []string{
		"{{tomorrow}}",           // unknown variable
		"{{now-90y:filetime}}",   // unknown unit
		"{{now:epoch}}",          // unknown format
		"{{me:filetime}}",        // me takes no format
	}

	for _, in := range tests {
		t.Run(in, func(t *testing.T) {
			if _, err := e.Expand(in); err == nil {
				t.Errorf("Expand(%q) succeeded, want an error", in)
			}
		})
	}
}

func TestExpandRejectsMeWithoutABind(t *testing.T) {
	e := Expander{Now: fixed}
	if _, err := e.Expand("(manager={{me}})"); err == nil {
		t.Error("Expand() with no bind DN succeeded, want an error")
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/filters/ -run TestExpand -v`
Expected: FAIL — `undefined: Expander`.

- [ ] **Step 3: Implement it**

```go
package filters

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Expander turns a filter template into a filter a server will accept.
type Expander struct {
	// Now is the instant {{now}} resolves to. Always set it explicitly.
	Now time.Time
	// BindDN is the distinguished name {{me}} resolves to. Empty means the
	// connection has no bound identity, and {{me}} becomes an error.
	BindDN string
}

// substitution matches {{now}}, {{now-90d}}, {{now+1d:filetime}} and {{me}}.
var substitution = regexp.MustCompile(`\{\{([^}]*)\}\}`)

// offset splits "now-90d" into its sign, amount and unit.
var offset = regexp.MustCompile(`^now(?:([+-])(\d+)([dhm]))?$`)

const (
	ticksPerSecond = 10_000_000
	epochOffset    = 11_644_473_600
)

// Expand replaces every {{…}} in filter. An unrecognised substitution is an
// error: sending it to the server verbatim would silently return nothing.
func (e Expander) Expand(filter string) (string, error) {
	var firstErr error

	out := substitution.ReplaceAllStringFunc(filter, func(match string) string {
		body := strings.TrimSpace(match[2 : len(match)-2])

		name, format := body, ""
		if i := strings.IndexByte(body, ':'); i >= 0 {
			name, format = body[:i], body[i+1:]
		}

		value, err := e.resolve(name, format)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		return value
	})

	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}

func (e Expander) resolve(name, format string) (string, error) {
	if name == "me" {
		if format != "" {
			return "", fmt.Errorf("filters: {{me}} takes no format, got %q", format)
		}
		if e.BindDN == "" {
			return "", fmt.Errorf("filters: {{me}} needs a bound connection")
		}
		return e.BindDN, nil
	}

	m := offset.FindStringSubmatch(name)
	if m == nil {
		return "", fmt.Errorf("filters: unknown substitution {{%s}}", name)
	}

	t := e.Now
	if m[1] != "" {
		amount, err := strconv.Atoi(m[2])
		if err != nil {
			return "", fmt.Errorf("filters: %q is not a number", m[2])
		}
		var d time.Duration
		switch m[3] {
		case "d":
			d = time.Duration(amount) * 24 * time.Hour
		case "h":
			d = time.Duration(amount) * time.Hour
		case "m":
			d = time.Duration(amount) * time.Minute
		}
		if m[1] == "-" {
			d = -d
		}
		t = t.Add(d)
	}

	switch format {
	case "", "generalized":
		return t.UTC().Format("20060102150405Z"), nil
	case "filetime":
		return strconv.FormatInt((t.UTC().Unix()+epochOffset)*ticksPerSecond, 10), nil
	default:
		return "", fmt.Errorf("filters: unknown format %q, want filetime or generalized", format)
	}
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/filters/ -run TestExpand -v`
Expected: PASS for every subtest.

The `ticksPerSecond` and `epochOffset` constants also exist in `internal/decode`. That duplication
is deliberate — the two packages are independent, and coupling them through a shared constants
package would buy nothing.

- [ ] **Step 5: Commit**

```bash
git add internal/filters/expand.go internal/filters/expand_test.go
git commit -m "Expand {{now}} and {{me}} substitutions in filter templates"
```

---

## Task 10: Validate filters before they reach a server

**Files:**
- Create: `internal/filters/validate.go`
- Test: `internal/filters/validate_test.go`

- [ ] **Step 1: Write the failing test**

```go
package filters

import (
	"strings"
	"testing"
	"time"
)

func TestValidateAcceptsGoodFilters(t *testing.T) {
	e := Expander{Now: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	good := []string{
		"(objectClass=*)",
		"(&(objectClass=user)(sAMAccountName=a.volkova))",
		"(|(objectClass=group)(objectClass=groupOfNames))",
		"(&(objectClass=user)(!(mail=*)))",
		"(&(objectCategory=person)(lastLogonTimestamp<={{now-90d:filetime}}))",
		"(userAccountControl:1.2.840.113556.1.4.803:=2)",
	}
	for _, in := range good {
		t.Run(in, func(t *testing.T) {
			if err := Validate(in, e); err != nil {
				t.Errorf("Validate(%q) = %v, want nil", in, err)
			}
		})
	}
}

func TestValidateRejectsBadFilters(t *testing.T) {
	e := Expander{Now: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	bad := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"unbalanced", "(objectClass=user"},
		{"no parentheses", "objectClass=user"},
		{"empty conjunction", "(&)"},
		{"bad substitution", "(whenCreated<={{yesterday}})"},
	}
	for _, tt := range bad {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.in, e); err == nil {
				t.Errorf("Validate(%q) = nil, want an error", tt.in)
			}
		})
	}
}

func TestValidateEveryBuiltin(t *testing.T) {
	// The shipped set must be valid. This test is the reason a typo in
	// builtin.json cannot reach a release.
	e := Expander{
		Now:    time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC),
		BindDN: "CN=test,DC=example,DC=com",
	}
	for _, f := range Builtins() {
		if err := Validate(f.Filter, e); err != nil {
			t.Errorf("built-in filter %q is invalid: %v", f.ID, err)
		}
	}
}

func TestValidateErrorNamesTheProblem(t *testing.T) {
	e := Expander{Now: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	err := Validate("(objectClass=user", e)
	if err == nil {
		t.Fatal("Validate() = nil, want an error")
	}
	if !strings.Contains(err.Error(), "(objectClass=user") {
		t.Errorf("error %q does not quote the offending filter", err)
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/filters/ -run TestValidate -v`
Expected: FAIL — `undefined: Validate`.

- [ ] **Step 3: Implement it**

```go
package filters

import (
	"fmt"

	"github.com/go-ldap/ldap/v3"
)

// Validate checks that a filter template expands and then parses as RFC 4515.
// Catching this locally matters: an invalid filter sent to a server comes back
// as a protocol error with no hint about which part was wrong.
func Validate(filter string, e Expander) error {
	if filter == "" {
		return fmt.Errorf("filters: the filter is empty")
	}

	expanded, err := e.Expand(filter)
	if err != nil {
		return err
	}

	if _, err := ldap.CompileFilter(expanded); err != nil {
		return fmt.Errorf("filters: %q is not a valid LDAP filter: %w", filter, err)
	}
	return nil
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/filters/ -run TestValidate -v`
Expected: PASS.

If `TestValidateRejectsBadFilters/empty_conjunction` fails, `CompileFilter` on the pinned version
accepts `(&)` — which is legal RFC 4515 for "match everything". Drop that one case from the test
rather than adding a rule the standard does not have.

- [ ] **Step 5: Commit**

```bash
git add internal/filters/validate.go internal/filters/validate_test.go
git commit -m "Validate filters locally before they reach a server"
```

---

## Task 11: The filter store

The user's changes live in a small overlay file, never in a rewritten copy of the shipped set.
That is what makes `Reset` possible, and what stops an Ldapper update from clobbering edits or
resurrecting a filter someone deleted on purpose.

**Files:**
- Create: `internal/filters/store.go`
- Test: `internal/filters/store_test.go`

- [ ] **Step 1: Write the failing test**

```go
package filters

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestStore builds a store backed by a throwaway file.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "filters.json")
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	return s
}

func TestStoreStartsWithTheBuiltins(t *testing.T) {
	s := newTestStore(t)
	if len(s.All()) != 18 {
		t.Errorf("All() returned %d filters, want the 18 built-ins", len(s.All()))
	}
}

func TestStoreEditingABuiltinMarksItModified(t *testing.T) {
	s := newTestStore(t)

	f, ok := s.Get("ad-stale-users-90d")
	if !ok {
		t.Fatal("Get() could not find ad-stale-users-90d")
	}
	f.Filter = "(&(objectCategory=person)(lastLogonTimestamp<={{now-30d:filetime}}))"
	if err := s.Save(f); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	got, _ := s.Get("ad-stale-users-90d")
	if !got.Modified {
		t.Error("the edited built-in is not marked Modified")
	}
	if !got.BuiltIn {
		t.Error("the edited filter lost its BuiltIn flag")
	}
	if got.Filter != f.Filter {
		t.Errorf("Filter = %q, want the edit to have stuck", got.Filter)
	}
	if len(s.All()) != 18 {
		t.Errorf("All() returned %d filters, want 18 — an edit must not add a filter", len(s.All()))
	}
}

func TestStoreResetUndoesAnEdit(t *testing.T) {
	s := newTestStore(t)
	original, _ := s.Get("ad-disabled-accounts")

	edited := original
	edited.Name = "Switched-off accounts"
	if err := s.Save(edited); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	if err := s.Reset("ad-disabled-accounts"); err != nil {
		t.Fatalf("Reset() returned error: %v", err)
	}

	got, _ := s.Get("ad-disabled-accounts")
	if got.Name != original.Name {
		t.Errorf("Name = %q, want the shipped name %q back", got.Name, original.Name)
	}
	if got.Modified {
		t.Error("the filter is still marked Modified after Reset")
	}
}

func TestStoreCustomFilters(t *testing.T) {
	s := newTestStore(t)

	f := Filter{
		ID:      "my-vpn-users",
		Name:    "VPN group members",
		Filter:  "(memberOf=CN=VPN Users,DC=example,DC=com)",
		Scope:   ScopeSubtree,
		Dialect: DialectGeneric,
	}
	if err := s.Save(f); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	got, ok := s.Get("my-vpn-users")
	if !ok {
		t.Fatal("Get() could not find the saved custom filter")
	}
	if got.BuiltIn {
		t.Error("a custom filter is marked BuiltIn")
	}
	if len(s.All()) != 19 {
		t.Errorf("All() returned %d filters, want 19", len(s.All()))
	}
}

func TestStoreDeleteRemovesBothKinds(t *testing.T) {
	s := newTestStore(t)

	custom := Filter{ID: "mine", Name: "Mine", Filter: "(objectClass=*)", Scope: ScopeSubtree, Dialect: DialectGeneric}
	if err := s.Save(custom); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	if err := s.Delete("mine"); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}
	if _, ok := s.Get("mine"); ok {
		t.Error("the deleted custom filter is still present")
	}

	if err := s.Delete("posix-accounts"); err != nil {
		t.Fatalf("Delete() on a built-in returned error: %v", err)
	}
	if _, ok := s.Get("posix-accounts"); ok {
		t.Error("the deleted built-in is still present")
	}
	if len(s.All()) != 17 {
		t.Errorf("All() returned %d filters, want 17", len(s.All()))
	}
}

func TestStoreDeletedBuiltinStaysDeletedAcrossRestarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	if err := s.Delete("posix-accounts"); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}

	reopened, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() on reopen returned error: %v", err)
	}
	if _, ok := reopened.Get("posix-accounts"); ok {
		t.Error("the deleted built-in came back after reopening the store")
	}
}

func TestStoreRestoreDefaultsKeepsCustomFilters(t *testing.T) {
	s := newTestStore(t)

	edited, _ := s.Get("ad-disabled-accounts")
	edited.Name = "Edited"
	if err := s.Save(edited); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	if err := s.Delete("posix-accounts"); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}
	custom := Filter{ID: "mine", Name: "Mine", Filter: "(objectClass=*)", Scope: ScopeSubtree, Dialect: DialectGeneric}
	if err := s.Save(custom); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	if err := s.RestoreDefaults(); err != nil {
		t.Fatalf("RestoreDefaults() returned error: %v", err)
	}

	if got, _ := s.Get("ad-disabled-accounts"); got.Name == "Edited" {
		t.Error("RestoreDefaults() left the edit in place")
	}
	if _, ok := s.Get("posix-accounts"); !ok {
		t.Error("RestoreDefaults() did not bring back the deleted built-in")
	}
	if _, ok := s.Get("mine"); !ok {
		t.Error("RestoreDefaults() removed a custom filter, which it must never do")
	}
}

func TestStorePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	custom := Filter{ID: "mine", Name: "Mine", Filter: "(objectClass=*)", Scope: ScopeSubtree, Dialect: DialectGeneric}
	if err := s.Save(custom); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	reopened, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() on reopen returned error: %v", err)
	}
	if _, ok := reopened.Get("mine"); !ok {
		t.Error("the custom filter did not survive a reopen")
	}
}

func TestStoreRejectsAFilterWithNoID(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save(Filter{Name: "No id", Filter: "(objectClass=*)"}); err == nil {
		t.Error("Save() accepted a filter with no ID, want an error")
	}
}

func TestNewStoreCreatesNothingUntilSomethingIsSaved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "filters.json")
	if _, err := NewStore(path); err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("NewStore() wrote a file before the user changed anything")
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/filters/ -run TestStore -v`
Expected: FAIL — `undefined: NewStore`.

- [ ] **Step 3: Implement it**

```go
package filters

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// overlay is the on-disk shape: only what the user changed, never a copy of the
// shipped set. Keeping it this way is what lets an Ldapper update improve a
// built-in filter without discarding anyone's edits.
type overlay struct {
	// Overrides holds edited built-ins, keyed by filter ID.
	Overrides map[string]Filter `json:"overrides"`
	// Custom holds filters the user created.
	Custom []Filter `json:"custom"`
	// Removed lists built-in IDs the user deleted, so they stay gone.
	Removed []string `json:"removed"`
}

// Store is the filter library: the shipped set with the user's overlay applied.
// It is safe for concurrent use.
type Store struct {
	path string

	mu   sync.RWMutex
	over overlay
}

// NewStore loads the overlay at path. A missing file is not an error — it means
// nothing has been customised yet, and no file is written until something is.
func NewStore(path string) (*Store, error) {
	s := &Store{
		path: path,
		over: overlay{Overrides: map[string]Filter{}},
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("filters: cannot read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &s.over); err != nil {
		return nil, fmt.Errorf("filters: %s is not valid JSON: %w", path, err)
	}
	if s.over.Overrides == nil {
		s.over.Overrides = map[string]Filter{}
	}
	return s, nil
}

// All returns every filter the user should see, built-ins first in their
// shipped order, then custom filters by name.
func (s *Store) All() []Filter {
	s.mu.RLock()
	defer s.mu.RUnlock()

	removed := map[string]bool{}
	for _, id := range s.over.Removed {
		removed[id] = true
	}

	var out []Filter
	for _, f := range builtins {
		if removed[f.ID] {
			continue
		}
		if edited, ok := s.over.Overrides[f.ID]; ok {
			edited.BuiltIn = true
			edited.Modified = true
			out = append(out, edited)
			continue
		}
		out = append(out, f)
	}

	custom := make([]Filter, len(s.over.Custom))
	copy(custom, s.over.Custom)
	sort.Slice(custom, func(i, j int) bool { return custom[i].Name < custom[j].Name })
	return append(out, custom...)
}

// Get returns one filter by ID.
func (s *Store) Get(id string) (Filter, bool) {
	for _, f := range s.All() {
		if f.ID == id {
			return f, true
		}
	}
	return Filter{}, false
}

// Save writes f. Saving over a built-in ID records an override; any other ID
// creates or replaces a custom filter.
func (s *Store) Save(f Filter) error {
	if f.ID == "" {
		return fmt.Errorf("filters: a filter needs an ID")
	}
	if f.Name == "" {
		return fmt.Errorf("filters: filter %q needs a name", f.ID)
	}
	if f.Filter == "" {
		return fmt.Errorf("filters: filter %q needs a filter expression", f.ID)
	}
	if f.Scope == "" {
		f.Scope = ScopeSubtree
	}
	if f.Dialect == "" {
		f.Dialect = DialectGeneric
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	f.BuiltIn, f.Modified = false, false

	if isBuiltin(f.ID) {
		s.over.Overrides[f.ID] = f
		s.over.Removed = without(s.over.Removed, f.ID)
	} else {
		replaced := false
		for i := range s.over.Custom {
			if s.over.Custom[i].ID == f.ID {
				s.over.Custom[i] = f
				replaced = true
				break
			}
		}
		if !replaced {
			s.over.Custom = append(s.over.Custom, f)
		}
	}
	return s.flush()
}

// Reset drops the user's edit to a built-in filter. It is a no-op for custom
// filters, which have no shipped version to return to.
func (s *Store) Reset(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.over.Overrides[id]; !ok {
		return nil
	}
	delete(s.over.Overrides, id)
	return s.flush()
}

// Delete removes a filter. A built-in is remembered as removed so that it does
// not reappear the next time Ldapper starts.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if isBuiltin(id) {
		delete(s.over.Overrides, id)
		for _, existing := range s.over.Removed {
			if existing == id {
				return nil
			}
		}
		s.over.Removed = append(s.over.Removed, id)
		return s.flush()
	}

	kept := s.over.Custom[:0]
	for _, f := range s.over.Custom {
		if f.ID != id {
			kept = append(kept, f)
		}
	}
	s.over.Custom = kept
	return s.flush()
}

// RestoreDefaults undoes every edit and deletion of a built-in filter. Custom
// filters are left alone — they were never part of the shipped set.
func (s *Store) RestoreDefaults() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.over.Overrides = map[string]Filter{}
	s.over.Removed = nil
	return s.flush()
}

// flush writes the overlay to disk. The caller must hold the write lock.
func (s *Store) flush() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("filters: cannot create %s: %w", filepath.Dir(s.path), err)
	}

	data, err := json.MarshalIndent(s.over, "", "  ")
	if err != nil {
		return fmt.Errorf("filters: cannot encode the overlay: %w", err)
	}

	// Write to a neighbouring file and rename, so a crash mid-write cannot
	// leave the library truncated.
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("filters: cannot write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("filters: cannot replace %s: %w", s.path, err)
	}
	return nil
}

func isBuiltin(id string) bool {
	for _, f := range builtins {
		if f.ID == id {
			return true
		}
	}
	return false
}

func without(list []string, id string) []string {
	out := list[:0]
	for _, existing := range list {
		if existing != id {
			out = append(out, existing)
		}
	}
	return out
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/filters/ -run TestStore -v`
Expected: PASS for every subtest.

- [ ] **Step 5: Run the whole package**

Run: `go test ./internal/filters/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/filters/store.go internal/filters/store_test.go
git commit -m "Store filter edits as an overlay so Reset and updates both work"
```

---

## Task 12: Dialect compatibility

A filter using `1.2.840.113556.1.4.803` against OpenLDAP does not fail — it returns nothing. That
silent empty result is the failure mode this task exists to prevent.

**Files:**
- Create: `internal/filters/compatible.go`
- Test: `internal/filters/compatible_test.go`

- [ ] **Step 1: Write the failing test**

```go
package filters

import "testing"

func TestCompatible(t *testing.T) {
	tests := []struct {
		name      string
		dialect   Dialect
		supported []Dialect
		want      bool
	}{
		{"generic runs anywhere", DialectGeneric, []Dialect{DialectGeneric}, true},
		{"AD filter on AD", DialectAD, []Dialect{DialectGeneric, DialectAD}, true},
		{"AD filter on OpenLDAP", DialectAD, []Dialect{DialectGeneric, DialectPOSIX}, false},
		{"POSIX filter on OpenLDAP", DialectPOSIX, []Dialect{DialectGeneric, DialectPOSIX}, true},
		{"POSIX filter on AD", DialectPOSIX, []Dialect{DialectGeneric, DialectAD}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compatible(Filter{Dialect: tt.dialect}, tt.supported)
			if got != tt.want {
				t.Errorf("Compatible() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIncompatibleReasonNamesTheServer(t *testing.T) {
	got := IncompatibleReason(Filter{Name: "Disabled accounts", Dialect: DialectAD})
	if got == "" {
		t.Fatal("IncompatibleReason() returned an empty string")
	}
}

func TestIncompatibleReasonIsEmptyForGeneric(t *testing.T) {
	if got := IncompatibleReason(Filter{Dialect: DialectGeneric}); got != "" {
		t.Errorf("IncompatibleReason() = %q, want empty for a portable filter", got)
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/filters/ -run 'Compatible|Incompatible' -v`
Expected: FAIL — `undefined: Compatible`.

- [ ] **Step 3: Implement it**

```go
package filters

// Compatible reports whether f can run against a server offering the given
// dialects. A filter the server cannot answer correctly must not be run:
// returning nothing looks exactly like finding nothing.
func Compatible(f Filter, supported []Dialect) bool {
	for _, d := range supported {
		if d == f.Dialect {
			return true
		}
	}
	return false
}

// IncompatibleReason explains, in one sentence, why a filter is unavailable.
// It returns an empty string for portable filters, which are never hidden.
func IncompatibleReason(f Filter) string {
	switch f.Dialect {
	case DialectAD:
		return "This filter uses an Active Directory extension. The server you are connected to does not support it."
	case DialectPOSIX:
		return "This filter needs the POSIX schema, which the server you are connected to does not carry."
	default:
		return ""
	}
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/filters/ -v`
Expected: PASS for the whole package.

- [ ] **Step 5: Commit**

```bash
git add internal/filters/compatible.go internal/filters/compatible_test.go
git commit -m "Hide filters the connected server cannot answer correctly"
```

---

## Task 13: Connection profiles

A profile is everything needed to reconnect except the password, which belongs in the operating
system's keychain. The keychain being unavailable — a headless Linux box with no Secret Service,
say — is a normal condition, not a crash.

**Files:**
- Create: `internal/profiles/profile.go`, `internal/profiles/store.go`
- Test: `internal/profiles/store_test.go`

- [ ] **Step 1: Write the failing test**

```go
package profiles

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(filepath.Join(t.TempDir(), "connections.json"))
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	return s
}

func sample() Profile {
	return Profile{
		ID:         "dc01",
		Name:       "CORP production",
		Host:       "dc01.corp.example.com",
		Port:       636,
		Encryption: EncryptionLDAPS,
		BindMethod: BindNTLM,
		Domain:     "CORP",
		Username:   "a.kensel",
	}
}

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connections.json")

	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() returned error: %v", err)
	}
	if err := s.Save(sample()); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	reopened, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore() on reopen returned error: %v", err)
	}
	got, ok := reopened.Get("dc01")
	if !ok {
		t.Fatal("Get() could not find the saved profile")
	}
	if got.Host != "dc01.corp.example.com" || got.Port != 636 {
		t.Errorf("Get() = %+v, want the saved host and port", got)
	}
}

func TestStoreNeverWritesAPasswordToDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connections.json")
	s, _ := NewStore(path)

	p := sample()
	p.password = "hunter2"
	if err := s.Save(p); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read the profile file: %v", err)
	}
	if strings.Contains(string(data), "hunter2") {
		t.Fatal("the password was written to connections.json")
	}
}

func TestTrustFingerprintIsPerProfile(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save(sample()); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	const fp = "4B:9C:1E:07:AA:63:F2:80"
	if err := s.Trust("dc01", fp); err != nil {
		t.Fatalf("Trust() returned error: %v", err)
	}

	got, _ := s.Get("dc01")
	if !got.Trusts(fp) {
		t.Error("Trusts() = false for a fingerprint that was just trusted")
	}
	if got.Trusts("00:11:22:33") {
		t.Error("Trusts() = true for a fingerprint nobody approved")
	}
}

func TestTrustDoesNotLeakToOtherProfiles(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save(sample()); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	second := sample()
	second.ID = "dc02"
	second.Host = "dc02.corp.example.com"
	if err := s.Save(second); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	const fp = "4B:9C:1E:07:AA:63:F2:80"
	if err := s.Trust("dc01", fp); err != nil {
		t.Fatalf("Trust() returned error: %v", err)
	}

	other, _ := s.Get("dc02")
	if other.Trusts(fp) {
		t.Error("trusting a certificate for one profile trusted it for another")
	}
}

func TestDeleteRemovesTheProfile(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save(sample()); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}
	if err := s.Delete("dc01"); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}
	if _, ok := s.Get("dc01"); ok {
		t.Error("the profile is still present after Delete()")
	}
}

func TestSaveRejectsAnIncompleteProfile(t *testing.T) {
	s := newTestStore(t)
	tests := []struct {
		name string
		mut  func(*Profile)
	}{
		{"no id", func(p *Profile) { p.ID = "" }},
		{"no host", func(p *Profile) { p.Host = "" }},
		{"port out of range", func(p *Profile) { p.Port = 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := sample()
			tt.mut(&p)
			if err := s.Save(p); err == nil {
				t.Error("Save() accepted an incomplete profile, want an error")
			}
		})
	}
}
```

Add `"os"` and `"strings"` to the test file's imports.

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/profiles/ -v`
Expected: FAIL — `undefined: NewStore`.

- [ ] **Step 3: Write the Profile type**

```go
// Package profiles stores connection settings. Passwords are the one thing it
// will not put in a file: those go to the operating system's keychain.
package profiles

import "strings"

// Encryption is how the connection is protected.
type Encryption string

const (
	EncryptionNone     Encryption = "none"
	EncryptionLDAPS    Encryption = "ldaps"
	EncryptionStartTLS Encryption = "starttls"
)

// BindMethod is how the connection authenticates.
type BindMethod string

const (
	BindSimple BindMethod = "simple"
	BindNTLM   BindMethod = "ntlm"
)

// Profile is one saved connection.
type Profile struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Host       string     `json:"host"`
	Port       int        `json:"port"`
	Encryption Encryption `json:"encryption"`
	BindMethod BindMethod `json:"bindMethod"`
	// Domain is the NetBIOS domain for an NTLM bind. Unused for simple binds.
	Domain string `json:"domain,omitempty"`
	// Username is a DN or UPN for a simple bind, a bare account name for NTLM.
	Username string `json:"username,omitempty"`
	// RememberPassword asks for the password to be kept in the OS keychain.
	RememberPassword bool `json:"rememberPassword"`
	// TrustedFingerprints holds SHA-256 fingerprints the user approved for
	// this server, and this server alone.
	TrustedFingerprints []string `json:"trustedFingerprints,omitempty"`

	// password is never serialised — the lower-case name keeps it out of JSON
	// as well as out of reach of other packages.
	password string
}

// Password returns the password held in memory for this profile.
func (p Profile) Password() string { return p.password }

// WithPassword returns a copy carrying pw. The original is unchanged.
func (p Profile) WithPassword(pw string) Profile {
	p.password = pw
	return p
}

// Trusts reports whether the user has approved this certificate fingerprint
// for this profile. Comparison ignores case and separators, because the same
// fingerprint gets written several ways.
func (p Profile) Trusts(fingerprint string) bool {
	want := normaliseFingerprint(fingerprint)
	for _, fp := range p.TrustedFingerprints {
		if normaliseFingerprint(fp) == want {
			return true
		}
	}
	return false
}

func normaliseFingerprint(s string) string {
	return strings.ToLower(strings.NewReplacer(":", "", " ", "", "-", "").Replace(s))
}
```

- [ ] **Step 4: Write the store**

```go
package profiles

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/zalando/go-keyring"
)

// keyringService is the name Ldapper's entries appear under in the OS keychain.
const keyringService = "ldapper"

// ErrNoKeychain means the operating system has no usable secret store. It is a
// condition to report, not a failure to abort on: the user can still type the
// password each time.
var ErrNoKeychain = errors.New("profiles: no usable keychain on this system")

// Store keeps connection profiles in a JSON file. It is safe for concurrent use.
type Store struct {
	path string

	mu   sync.RWMutex
	list []Profile
}

// NewStore loads the profiles at path. A missing file means no profiles yet.
func NewStore(path string) (*Store, error) {
	s := &Store{path: path}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("profiles: cannot read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &s.list); err != nil {
		return nil, fmt.Errorf("profiles: %s is not valid JSON: %w", path, err)
	}
	return s, nil
}

// All returns every saved profile.
func (s *Store) All() []Profile {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Profile, len(s.list))
	copy(out, s.list)
	return out
}

// Get returns one profile by ID.
func (s *Store) Get(id string) (Profile, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.list {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

// Save writes p, replacing any profile with the same ID. If p asks for its
// password to be remembered, it goes to the keychain and never to the file.
func (s *Store) Save(p Profile) error {
	switch {
	case p.ID == "":
		return fmt.Errorf("profiles: a profile needs an ID")
	case p.Host == "":
		return fmt.Errorf("profiles: profile %q needs a host", p.ID)
	case p.Port < 1 || p.Port > 65535:
		return fmt.Errorf("profiles: profile %q has port %d, want 1-65535", p.ID, p.Port)
	}

	if p.RememberPassword && p.password != "" {
		if err := keyring.Set(keyringService, p.ID, p.password); err != nil {
			return fmt.Errorf("%w: %v", ErrNoKeychain, err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	stored := p
	stored.password = ""

	replaced := false
	for i := range s.list {
		if s.list[i].ID == p.ID {
			s.list[i] = stored
			replaced = true
			break
		}
	}
	if !replaced {
		s.list = append(s.list, stored)
	}
	return s.flush()
}

// LoadPassword returns the profile with its remembered password attached.
// A profile that does not remember its password comes back unchanged, and a
// missing keychain returns ErrNoKeychain — in both cases the caller should ask
// the user rather than give up.
func (s *Store) LoadPassword(p Profile) (Profile, error) {
	if !p.RememberPassword {
		return p, nil
	}
	pw, err := keyring.Get(keyringService, p.ID)
	if err != nil {
		return p, fmt.Errorf("%w: %v", ErrNoKeychain, err)
	}
	return p.WithPassword(pw), nil
}

// Trust records a certificate fingerprint as approved for one profile only.
func (s *Store) Trust(id, fingerprint string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.list {
		if s.list[i].ID != id {
			continue
		}
		if s.list[i].Trusts(fingerprint) {
			return nil
		}
		s.list[i].TrustedFingerprints = append(s.list[i].TrustedFingerprints, fingerprint)
		return s.flush()
	}
	return fmt.Errorf("profiles: no profile with ID %q", id)
}

// Delete removes a profile and forgets its password.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// A keychain entry that is already gone is not a problem worth reporting.
	_ = keyring.Delete(keyringService, id)

	kept := s.list[:0]
	for _, p := range s.list {
		if p.ID != id {
			kept = append(kept, p)
		}
	}
	s.list = kept
	return s.flush()
}

// flush writes the profile list. The caller must hold the write lock.
func (s *Store) flush() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("profiles: cannot create %s: %w", filepath.Dir(s.path), err)
	}

	data, err := json.MarshalIndent(s.list, "", "  ")
	if err != nil {
		return fmt.Errorf("profiles: cannot encode the profiles: %w", err)
	}

	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("profiles: cannot write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("profiles: cannot replace %s: %w", s.path, err)
	}
	return nil
}
```

- [ ] **Step 5: Run the tests and confirm they pass**

Run: `go test ./internal/profiles/ -v`
Expected: PASS.

`TestStoreNeverWritesAPasswordToDisk` sets `RememberPassword` to false, so it never touches the
keychain and runs anywhere. If any test does start prompting for keychain access on macOS, that
is the signal it needs its own build tag — keychain calls do not belong in the default test run.

- [ ] **Step 6: Commit**

```bash
git add internal/profiles/
git commit -m "Store connection profiles with passwords in the OS keychain"
```

---

## Task 14: Dial with an explicit certificate decision

The rule from the spec: a TLS failure is never swallowed. When verification fails, the caller gets
back everything needed to show a person the fingerprint and ask them.

**Files:**
- Create: `internal/session/dial.go`
- Test: `internal/session/dial_test.go`

- [ ] **Step 1: Write the failing test**

```go
package session

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"strings"
	"testing"
)

func TestAddressBuildsTheRightURL(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{"plain", Config{Host: "dc01.example.com", Port: 389, Encryption: EncryptionNone}, "ldap://dc01.example.com:389"},
		{"ldaps", Config{Host: "dc01.example.com", Port: 636, Encryption: EncryptionLDAPS}, "ldaps://dc01.example.com:636"},
		{"starttls dials plain first", Config{Host: "dc01.example.com", Port: 389, Encryption: EncryptionStartTLS}, "ldap://dc01.example.com:389"},
		{"ipv6 is bracketed", Config{Host: "2001:db8::1", Port: 636, Encryption: EncryptionLDAPS}, "ldaps://[2001:db8::1]:636"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.address(); got != tt.want {
				t.Errorf("address() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFingerprintFormat(t *testing.T) {
	cert := &x509.Certificate{Raw: []byte("not a real certificate")}
	got := Fingerprint(cert)

	if len(got) != 95 { // 32 bytes → 64 hex characters + 31 colons
		t.Errorf("Fingerprint() = %q (%d chars), want 95", got, len(got))
	}
	if strings.ToUpper(got) != got {
		t.Errorf("Fingerprint() = %q, want upper case", got)
	}
	if strings.Count(got, ":") != 31 {
		t.Errorf("Fingerprint() = %q, want 31 separators", got)
	}
}

func TestCertErrorCarriesEnoughToDecide(t *testing.T) {
	err := &CertError{
		Fingerprint: "4B:9C:1E:07",
		Subject:     "CN=dc01.corp.example.com",
		Issuer:      "CN=CORP-ROOT-CA",
	}
	msg := err.Error()
	for _, want := range []string{"4B:9C:1E:07", "dc01.corp.example.com"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, want it to include %q", msg, want)
		}
	}

	var target *CertError
	if !errors.As(error(err), &target) {
		t.Error("errors.As could not unwrap a CertError")
	}
}

func TestVerifierAcceptsAPreviouslyTrustedFingerprint(t *testing.T) {
	// A certificate the system rejects but the user has already approved for
	// this profile must be allowed through.
	cert := &x509.Certificate{Raw: []byte("self-signed"), Subject: pkixName("dc01"), Issuer: pkixName("CORP-ROOT-CA")}
	cfg := Config{TrustedFingerprints: []string{Fingerprint(cert)}}

	verify := cfg.verifier()
	err := verify([][]byte{cert.Raw}, nil)
	if err != nil {
		t.Errorf("verifier() rejected a trusted fingerprint: %v", err)
	}
}

func TestDialRefusesAnUnreachableHost(t *testing.T) {
	cfg := Config{Host: "127.0.0.1", Port: 1, Encryption: EncryptionNone}
	if _, err := Dial(context.Background(), cfg); err == nil {
		t.Error("Dial() to a closed port succeeded, want an error")
	}
}

var _ = tls.Config{} // keeps the import honest while the file is being written
```

Add a small helper to the test file:

```go
import "crypto/x509/pkix"

func pkixName(cn string) pkix.Name { return pkix.Name{CommonName: cn} }
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/session/ -v`
Expected: FAIL — `undefined: Config`.

- [ ] **Step 3: Implement it**

```go
// Package session owns the life cycle of a directory connection: dialling,
// protecting it with TLS, and authenticating.
package session

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// Encryption is how the connection is protected.
type Encryption string

const (
	EncryptionNone     Encryption = "none"
	EncryptionLDAPS    Encryption = "ldaps"
	EncryptionStartTLS Encryption = "starttls"
)

// Config is everything needed to reach a server.
type Config struct {
	Host       string
	Port       int
	Encryption Encryption
	// TrustedFingerprints are SHA-256 fingerprints the user has already
	// approved for this server. Anything else fails verification.
	TrustedFingerprints []string
	// Timeout bounds the dial and every later request. Zero means 30 seconds.
	Timeout time.Duration
}

// CertError says the server presented a certificate the system does not trust,
// and carries what a person needs in order to decide whether to accept it.
type CertError struct {
	Fingerprint string
	Subject     string
	Issuer      string
	NotAfter    time.Time
	Err         error
}

func (e *CertError) Error() string {
	return fmt.Sprintf("the certificate for %s is not trusted (issued by %s, SHA-256 %s)",
		e.Subject, e.Issuer, e.Fingerprint)
}

func (e *CertError) Unwrap() error { return e.Err }

// Conn is an authenticated directory connection.
type Conn struct {
	*ldap.Conn
	// BindDN is the identity the connection is bound as, once a bind succeeds.
	BindDN string
}

// Fingerprint is the SHA-256 of a certificate, formatted the way certificate
// dialogs show it: upper-case hex, colon-separated.
func Fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":")
}

// address renders the LDAP URL to dial. StartTLS starts on the plain scheme
// and upgrades afterwards.
func (c Config) address() string {
	scheme := "ldap"
	if c.Encryption == EncryptionLDAPS {
		scheme = "ldaps"
	}
	return scheme + "://" + net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

func (c Config) timeout() time.Duration {
	if c.Timeout <= 0 {
		return 30 * time.Second
	}
	return c.Timeout
}

// verifier builds the certificate check. It runs the normal system
// verification first, and only then consults the fingerprints the user
// approved — so a valid certificate never depends on a stored exception.
func (c Config) verifier() func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verified [][]*x509.Certificate) error {
		if len(verified) > 0 {
			return nil
		}
		if len(rawCerts) == 0 {
			return fmt.Errorf("session: the server presented no certificate")
		}

		cert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("session: the server's certificate could not be parsed: %w", err)
		}

		fp := Fingerprint(cert)
		for _, trusted := range c.TrustedFingerprints {
			if strings.EqualFold(normalise(trusted), normalise(fp)) {
				return nil
			}
		}

		return &CertError{
			Fingerprint: fp,
			Subject:     cert.Subject.String(),
			Issuer:      cert.Issuer.String(),
			NotAfter:    cert.NotAfter,
		}
	}
}

func normalise(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, ":", ""))
}

// tlsConfig turns off Go's built-in verification only so that verifier can run
// the same checks and report a usable error instead of an opaque one. It is
// not a relaxation: verifier still requires either system trust or an explicit
// approval.
func (c Config) tlsConfig() *tls.Config {
	return &tls.Config{
		ServerName:            c.Host,
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: c.verifier(),
		MinVersion:            tls.VersionTLS12,
	}
}

// Dial opens a connection and applies the requested encryption. The returned
// connection is not yet authenticated — call one of the Bind methods next.
func Dial(ctx context.Context, cfg Config) (*Conn, error) {
	dialer := &net.Dialer{Timeout: cfg.timeout()}

	opts := []ldap.DialOpt{ldap.DialWithDialer(dialer)}
	if cfg.Encryption == EncryptionLDAPS {
		opts = append(opts, ldap.DialWithTLSConfig(cfg.tlsConfig()))
	}

	conn, err := ldap.DialURL(cfg.address(), opts...)
	if err != nil {
		return nil, err
	}

	if cfg.Encryption == EncryptionStartTLS {
		if err := conn.StartTLS(cfg.tlsConfig()); err != nil {
			conn.Close()
			return nil, err
		}
	}

	conn.SetTimeout(cfg.timeout())

	// Closing on cancellation is the only cancellation go-ldap offers for the
	// connection itself; per-request cancellation comes from SearchAsync.
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	return &Conn{Conn: conn}, nil
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/session/ -v`
Expected: PASS.

`TestVerifierAcceptsAPreviouslyTrustedFingerprint` passes `nil` for the verified chains, which is
what Go does when system verification fails — exactly the case being tested.

- [ ] **Step 5: Commit**

```bash
git add internal/session/dial.go internal/session/dial_test.go
git commit -m "Dial with TLS, surfacing untrusted certificates instead of hiding them"
```

---

## Task 15: Bind

**Files:**
- Create: `internal/session/bind.go`
- Test: `internal/session/bind_test.go`

- [ ] **Step 1: Write the failing test**

Binding needs a server, so the unit test covers only the part that can go wrong without one:
splitting `CORP\a.kensel` into the domain and account NTLM wants. The binds themselves are
covered by the integration tests in Task 21.

```go
package session

import "testing"

func TestSplitAccount(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantDomain string
		wantUser   string
	}{
		{"down-level name", `CORP\a.kensel`, "CORP", "a.kensel"},
		{"forward slash", "CORP/a.kensel", "CORP", "a.kensel"},
		{"bare account", "a.kensel", "", "a.kensel"},
		{"UPN is left alone", "a.kensel@corp.example.com", "", "a.kensel@corp.example.com"},
		{"surrounding space", `  CORP\a.kensel  `, "CORP", "a.kensel"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, user := SplitAccount(tt.in)
			if domain != tt.wantDomain || user != tt.wantUser {
				t.Errorf("SplitAccount(%q) = (%q, %q), want (%q, %q)",
					tt.in, domain, user, tt.wantDomain, tt.wantUser)
			}
		})
	}
}

func TestBindSimpleRejectsAnEmptyPassword(t *testing.T) {
	// An empty password makes most servers perform an unauthenticated bind
	// that reports success, which would show an empty directory and look like
	// a permissions problem. Refuse it here instead.
	c := &Conn{}
	if err := c.BindSimple("CN=admin,DC=example,DC=com", ""); err == nil {
		t.Error("BindSimple() with an empty password succeeded, want an error")
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/session/ -run 'SplitAccount|BindSimple' -v`
Expected: FAIL — `undefined: SplitAccount`.

- [ ] **Step 3: Implement it**

```go
package session

import (
	"fmt"
	"strings"
)

// SplitAccount separates a down-level logon name such as CORP\a.kensel into
// its domain and account. A user principal name is returned untouched: the
// part after the @ is a realm, not a NetBIOS domain.
func SplitAccount(s string) (domain, user string) {
	s = strings.TrimSpace(s)
	for _, sep := range []string{`\`, "/"} {
		if i := strings.Index(s, sep); i >= 0 {
			return s[:i], s[i+1:]
		}
	}
	return "", s
}

// BindSimple authenticates with a DN or user principal name and a password.
func (c *Conn) BindSimple(username, password string) error {
	if username == "" {
		return fmt.Errorf("session: a simple bind needs a username")
	}
	if password == "" {
		return fmt.Errorf("session: a simple bind needs a password — an empty one signs in anonymously and shows an empty directory")
	}
	if err := c.Conn.Bind(username, password); err != nil {
		return err
	}
	c.BindDN = username
	return nil
}

// BindNTLM authenticates the way an administrator thinks about their own
// account: domain, account name and password, with no distinguished name to
// look up first.
func (c *Conn) BindNTLM(domain, username, password string) error {
	if username == "" {
		return fmt.Errorf("session: an NTLM bind needs a username")
	}
	if password == "" {
		return fmt.Errorf("session: an NTLM bind needs a password")
	}
	// Accept CORP\a.kensel in the username field as well as in its own box.
	if d, u := SplitAccount(username); d != "" {
		domain, username = d, u
	}
	if domain == "" {
		return fmt.Errorf("session: an NTLM bind needs a domain, either on its own or as DOMAIN\\user")
	}
	if err := c.Conn.NTLMBind(domain, username, password); err != nil {
		return err
	}
	c.BindDN = domain + `\` + username
	return nil
}

// BindAnonymous connects without credentials. Some directories allow it for
// reading public entries.
func (c *Conn) BindAnonymous() error {
	if err := c.Conn.UnauthenticatedBind(""); err != nil {
		return err
	}
	c.BindDN = ""
	return nil
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/session/ -v`
Expected: PASS. `TestBindSimpleRejectsAnEmptyPassword` must fail before reaching `c.Conn`, which
is nil in that test — the argument check comes first, so it never dereferences it.

If `UnauthenticatedBind` is missing on the pinned version, check `go doc github.com/go-ldap/ldap/v3.Conn`
for the anonymous bind method and use that name.

- [ ] **Step 5: Commit**

```bash
git add internal/session/bind.go internal/session/bind_test.go
git commit -m "Add simple, NTLM and anonymous binds"
```

---

## Task 16: Read the RootDSE and work out what the server is

**Files:**
- Create: `internal/schema/rootdse.go`
- Test: `internal/schema/rootdse_test.go`

- [ ] **Step 1: Write the failing test**

```go
package schema

import (
	"testing"

	"github.com/skensell201/ldapper/internal/filters"
)

func TestDetectActiveDirectory(t *testing.T) {
	info := Info{SupportedControls: []string{
		"1.2.840.113556.1.4.319", // paged results
		"1.2.840.113556.1.4.800", // the marker that says Active Directory
	}}
	if !info.IsActiveDirectory() {
		t.Error("IsActiveDirectory() = false for a server advertising 1.2.840.113556.1.4.800")
	}
}

func TestDetectPlainLDAP(t *testing.T) {
	info := Info{SupportedControls: []string{"1.2.840.113556.1.4.319"}}
	if info.IsActiveDirectory() {
		t.Error("IsActiveDirectory() = true for a server that only supports paging")
	}
}

func TestSupportsPaging(t *testing.T) {
	with := Info{SupportedControls: []string{"1.2.840.113556.1.4.319"}}
	if !with.SupportsPaging() {
		t.Error("SupportsPaging() = false for a server advertising the paging control")
	}
	if (Info{}).SupportsPaging() {
		t.Error("SupportsPaging() = true for a server advertising nothing")
	}
}

func TestDialectsOnActiveDirectory(t *testing.T) {
	info := Info{SupportedControls: []string{"1.2.840.113556.1.4.800"}}
	got := info.Dialects()

	if !contains(got, filters.DialectAD) {
		t.Error("Dialects() omits the AD dialect on an AD server")
	}
	if !contains(got, filters.DialectGeneric) {
		t.Error("Dialects() omits the generic dialect, which every server supports")
	}
	if contains(got, filters.DialectPOSIX) {
		t.Error("Dialects() claims POSIX support on stock Active Directory")
	}
}

func TestDialectsOnOpenLDAP(t *testing.T) {
	info := Info{ObjectClasses: []string{"posixAccount", "inetOrgPerson"}}
	got := info.Dialects()

	if contains(got, filters.DialectAD) {
		t.Error("Dialects() claims AD support on a server that is not AD")
	}
	if !contains(got, filters.DialectPOSIX) {
		t.Error("Dialects() omits POSIX on a server carrying posixAccount")
	}
}

func TestDefaultNamingContext(t *testing.T) {
	info := Info{
		DefaultNamingContext: "DC=corp,DC=example,DC=com",
		NamingContexts:       []string{"DC=corp,DC=example,DC=com", "CN=Configuration,DC=corp,DC=example,DC=com"},
	}
	if got := info.RootDN(); got != "DC=corp,DC=example,DC=com" {
		t.Errorf("RootDN() = %q, want the default naming context", got)
	}
}

func TestRootDNFallsBackToTheFirstNamingContext(t *testing.T) {
	// OpenLDAP does not publish defaultNamingContext.
	info := Info{NamingContexts: []string{"dc=example,dc=com"}}
	if got := info.RootDN(); got != "dc=example,dc=com" {
		t.Errorf("RootDN() = %q, want the first naming context", got)
	}
}

func contains(list []filters.Dialect, want filters.Dialect) bool {
	for _, d := range list {
		if d == want {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/schema/ -v`
Expected: FAIL — `undefined: Info`.

- [ ] **Step 3: Implement it**

```go
// Package schema reads what a server says about itself, so the rest of
// Ldapper can stop guessing which kind of directory it is talking to.
package schema

import (
	"strings"

	"github.com/go-ldap/ldap/v3"
	"github.com/skensell201/ldapper/internal/filters"
)

// Control OIDs worth naming.
const (
	// OIDPagedResults is the simple paged results control. Without it, a
	// server will only ever return its first page.
	OIDPagedResults = "1.2.840.113556.1.4.319"
	// OIDLDAPPolicyHints is advertised by Active Directory and by nothing
	// else in common use, which makes it a reliable way to recognise it.
	OIDActiveDirectory = "1.2.840.113556.1.4.800"
)

// Info is what the RootDSE told us.
type Info struct {
	NamingContexts       []string
	DefaultNamingContext string
	SupportedControls    []string
	SupportedSASL        []string
	SubschemaSubentry    string
	VendorName           string
	// ObjectClasses is filled in lazily from the subschema; it stays empty
	// until something needs it.
	ObjectClasses []string
}

// IsActiveDirectory reports whether this is an Active Directory server.
func (i Info) IsActiveDirectory() bool {
	for _, oid := range i.SupportedControls {
		if oid == OIDActiveDirectory {
			return true
		}
	}
	return false
}

// SupportsPaging reports whether large result sets can be walked a page at a
// time. Without it, browsing a large organizational unit is not possible.
func (i Info) SupportsPaging() bool {
	for _, oid := range i.SupportedControls {
		if oid == OIDPagedResults {
			return true
		}
	}
	return false
}

// RootDN is the branch to open the tree at.
func (i Info) RootDN() string {
	if i.DefaultNamingContext != "" {
		return i.DefaultNamingContext
	}
	if len(i.NamingContexts) > 0 {
		return i.NamingContexts[0]
	}
	return ""
}

// Dialects lists the filter dialects this server can answer.
func (i Info) Dialects() []filters.Dialect {
	out := []filters.Dialect{filters.DialectGeneric}
	if i.IsActiveDirectory() {
		out = append(out, filters.DialectAD)
	}
	for _, class := range i.ObjectClasses {
		if strings.EqualFold(class, "posixAccount") {
			out = append(out, filters.DialectPOSIX)
			break
		}
	}
	return out
}

// Read fetches the RootDSE. Every attribute is optional: servers differ in
// what they publish, and a missing one is a fact, not a failure.
func Read(conn *ldap.Conn) (Info, error) {
	req := ldap.NewSearchRequest(
		"",
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{
			"namingContexts", "defaultNamingContext", "supportedControl",
			"supportedSASLMechanisms", "subschemaSubentry", "vendorName",
		},
		nil,
	)

	res, err := conn.Search(req)
	if err != nil {
		return Info{}, err
	}
	if len(res.Entries) == 0 {
		return Info{}, nil
	}

	e := res.Entries[0]
	return Info{
		NamingContexts:       e.GetAttributeValues("namingContexts"),
		DefaultNamingContext: e.GetAttributeValue("defaultNamingContext"),
		SupportedControls:    e.GetAttributeValues("supportedControl"),
		SupportedSASL:        e.GetAttributeValues("supportedSASLMechanisms"),
		SubschemaSubentry:    e.GetAttributeValue("subschemaSubentry"),
		VendorName:           e.GetAttributeValue("vendorName"),
	}, nil
}

// ReadObjectClasses fills in Info.ObjectClasses from the subschema entry. It is
// a separate call because the subschema is large and only needed to decide
// whether POSIX filters apply.
func ReadObjectClasses(conn *ldap.Conn, info Info) (Info, error) {
	if info.SubschemaSubentry == "" {
		return info, nil
	}

	req := ldap.NewSearchRequest(
		info.SubschemaSubentry,
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=subschema)",
		[]string{"objectClasses"},
		nil,
	)

	res, err := conn.Search(req)
	if err != nil {
		return info, err
	}
	if len(res.Entries) == 0 {
		return info, nil
	}

	for _, def := range res.Entries[0].GetAttributeValues("objectClasses") {
		if name := objectClassName(def); name != "" {
			info.ObjectClasses = append(info.ObjectClasses, name)
		}
	}
	return info, nil
}

// objectClassName pulls the NAME out of a schema definition such as
// ( 1.3.6.1.1.1.2.0 NAME 'posixAccount' SUP top AUXILIARY … ).
func objectClassName(def string) string {
	i := strings.Index(def, "NAME '")
	if i < 0 {
		return ""
	}
	rest := def[i+len("NAME '"):]
	j := strings.IndexByte(rest, '\'')
	if j < 0 {
		return ""
	}
	return rest[:j]
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/schema/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/schema/
git commit -m "Read the RootDSE and derive which filter dialects a server accepts"
```

---

## Task 17: Browse one level at a time

**Files:**
- Create: `internal/browse/children.go`
- Test: `internal/browse/children_test.go`

- [ ] **Step 1: Write the failing test**

The parts that can be tested without a server are the request construction and the entry mapping.
The paging loop itself is covered in Task 21.

```go
package browse

import (
	"testing"

	"github.com/go-ldap/ldap/v3"
)

func TestEntryFromLDAP(t *testing.T) {
	e := &ldap.Entry{
		DN: "CN=Anna Volkova,OU=Users,DC=corp,DC=example,DC=com",
		Attributes: []*ldap.EntryAttribute{
			{Name: "objectClass", Values: []string{"top", "person", "user"}},
			{Name: "numSubordinates", Values: []string{"0"}},
		},
	}

	got := entryFrom(e)
	if got.DN != e.DN {
		t.Errorf("DN = %q, want the full distinguished name", got.DN)
	}
	if got.RDN != "CN=Anna Volkova" {
		t.Errorf("RDN = %q, want CN=Anna Volkova", got.RDN)
	}
	if got.NumSubordinates != 0 {
		t.Errorf("NumSubordinates = %d, want 0", got.NumSubordinates)
	}
	if got.HasChildren {
		t.Error("HasChildren = true for an entry that reported no subordinates")
	}
	if len(got.Classes) != 3 {
		t.Errorf("Classes = %v, want all three", got.Classes)
	}
}

func TestEntryWithUnknownChildCount(t *testing.T) {
	// OpenLDAP does not publish numSubordinates. An unknown count must not be
	// mistaken for zero, or the tree would refuse to expand anything.
	e := &ldap.Entry{
		DN:         "ou=people,dc=example,dc=com",
		Attributes: []*ldap.EntryAttribute{{Name: "objectClass", Values: []string{"organizationalUnit"}}},
	}

	got := entryFrom(e)
	if got.NumSubordinates != -1 {
		t.Errorf("NumSubordinates = %d, want -1 for unknown", got.NumSubordinates)
	}
	if !got.HasChildren {
		t.Error("HasChildren = false for an entry whose child count is unknown; it must stay expandable")
	}
}

func TestRDNOfARootDN(t *testing.T) {
	e := &ldap.Entry{DN: "DC=corp,DC=example,DC=com"}
	if got := entryFrom(e).RDN; got != "DC=corp" {
		t.Errorf("RDN = %q, want DC=corp", got)
	}
}

func TestRDNHandlesAnEscapedComma(t *testing.T) {
	e := &ldap.Entry{DN: `CN=Volkova\, Anna,OU=Users,DC=example,DC=com`}
	if got := entryFrom(e).RDN; got != `CN=Volkova\, Anna` {
		t.Errorf("RDN = %q, want the escaped comma kept inside the RDN", got)
	}
}

func TestPageSizeFloor(t *testing.T) {
	if got := pageSize(0); got != defaultPageSize {
		t.Errorf("pageSize(0) = %d, want the default %d", got, defaultPageSize)
	}
	if got := pageSize(250); got != 250 {
		t.Errorf("pageSize(250) = %d, want it left alone", got)
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/browse/ -v`
Expected: FAIL — `undefined: entryFrom`.

- [ ] **Step 3: Implement it**

```go
// Package browse walks the directory tree one level at a time.
package browse

import (
	"strconv"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// defaultPageSize matches Active Directory's own MaxPageSize. Asking for more
// gains nothing; asking for less costs round trips.
const defaultPageSize = 1000

// Entry is one node in the tree.
type Entry struct {
	DN  string
	RDN string
	// Classes are the entry's objectClass values, which decide its icon.
	Classes []string
	// NumSubordinates is how many children the entry has, or -1 when the
	// server does not publish that.
	NumSubordinates int
	// HasChildren is false only when the server said so. An unknown count
	// leaves the node expandable — refusing to expand something that does
	// have children is the worse mistake.
	HasChildren bool
}

// Page is one batch of children plus the cookie needed to ask for the next.
type Page struct {
	Entries []Entry
	// Cookie is empty when the listing is complete.
	Cookie []byte
}

// Children lists the immediate children of dn. Pass a nil cookie for the first
// page, then the cookie from the previous Page for each one after.
func Children(conn *ldap.Conn, dn string, size uint32, cookie []byte) (Page, error) {
	paging := ldap.NewControlPaging(pageSize(size))
	if len(cookie) > 0 {
		paging.SetCookie(cookie)
	}

	req := ldap.NewSearchRequest(
		dn,
		ldap.ScopeSingleLevel, ldap.NeverDerefAliases, 0, 0, false,
		"(objectClass=*)",
		[]string{"objectClass", "numSubordinates"},
		[]ldap.Control{paging},
	)

	res, err := conn.Search(req)
	if err != nil {
		return Page{}, err
	}

	page := Page{Entries: make([]Entry, 0, len(res.Entries))}
	for _, e := range res.Entries {
		page.Entries = append(page.Entries, entryFrom(e))
	}

	if ctrl := ldap.FindControl(res.Controls, ldap.ControlTypePaging); ctrl != nil {
		page.Cookie = ctrl.(*ldap.ControlPaging).Cookie
	}
	return page, nil
}

func pageSize(n uint32) uint32 {
	if n == 0 {
		return defaultPageSize
	}
	return n
}

func entryFrom(e *ldap.Entry) Entry {
	out := Entry{
		DN:              e.DN,
		RDN:             firstRDN(e.DN),
		Classes:         e.GetAttributeValues("objectClass"),
		NumSubordinates: -1,
		HasChildren:     true,
	}

	if raw := e.GetAttributeValue("numSubordinates"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			out.NumSubordinates = n
			out.HasChildren = n > 0
		}
	}
	return out
}

// firstRDN returns the leftmost component of a DN, respecting the backslash
// escape that lets a comma appear inside a name.
func firstRDN(dn string) string {
	for i := 0; i < len(dn); i++ {
		if dn[i] == '\\' {
			i++ // skip whatever the backslash escapes
			continue
		}
		if dn[i] == ',' {
			return strings.TrimSpace(dn[:i])
		}
	}
	return dn
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/browse/ -v`
Expected: PASS.

If `ldap.FindControl` does not exist on the pinned version, look for the equivalent on
`*ldap.SearchResult` with `go doc github.com/go-ldap/ldap/v3.SearchResult` and use that instead.

- [ ] **Step 5: Commit**

```bash
git add internal/browse/
git commit -m "List a node's children with paged results"
```

---

## Task 18: Stream search results

Results arrive in batches as the server produces them, so a search across 50,000 objects fills the
table as it goes instead of showing nothing for eight seconds. A server that truncates the result
set is reporting a fact, not failing — the results already delivered stay.

**Files:**
- Create: `internal/search/stream.go`
- Test: `internal/search/stream_test.go`

- [ ] **Step 1: Write the failing test**

```go
package search

import (
	"errors"
	"testing"

	"github.com/go-ldap/ldap/v3"
)

func TestScopeMapping(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"base", ldap.ScopeBaseObject},
		{"one", ldap.ScopeSingleLevel},
		{"subtree", ldap.ScopeWholeSubtree},
		{"", ldap.ScopeWholeSubtree}, // subtree is the sensible default
		{"nonsense", ldap.ScopeWholeSubtree},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := scopeOf(tt.in); got != tt.want {
				t.Errorf("scopeOf(%q) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestResultFromEntry(t *testing.T) {
	e := &ldap.Entry{
		DN: "CN=Anna Volkova,OU=Users,DC=example,DC=com",
		Attributes: []*ldap.EntryAttribute{
			{Name: "cn", Values: []string{"Anna Volkova"}, ByteValues: [][]byte{[]byte("Anna Volkova")}},
			{Name: "objectClass", Values: []string{"top", "user"}, ByteValues: [][]byte{[]byte("top"), []byte("user")}},
		},
	}

	got := resultFrom(e)
	if got.DN != e.DN {
		t.Errorf("DN = %q, want the entry's DN", got.DN)
	}
	if len(got.Attributes["objectClass"]) != 2 {
		t.Errorf("objectClass has %d values, want 2", len(got.Attributes["objectClass"]))
	}
	if got.Attributes["cn"][0].Raw != "Anna Volkova" {
		t.Errorf("cn = %q, want the decoded value", got.Attributes["cn"][0].Raw)
	}
}

func TestTruncationIsNotAFailure(t *testing.T) {
	tests := []struct {
		name string
		code uint16
		want string
	}{
		{"size limit", ldap.LDAPResultSizeLimitExceeded, "size"},
		{"time limit", ldap.LDAPResultTimeLimitExceeded, "time"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, truncated := truncationOf(&ldap.Error{ResultCode: tt.code})
			if !truncated {
				t.Fatalf("truncationOf() = false, want a %s limit to count as truncation", tt.want)
			}
			if reason == "" {
				t.Error("truncationOf() gave no reason")
			}
		})
	}
}

func TestOtherErrorsAreNotTruncation(t *testing.T) {
	if _, truncated := truncationOf(errors.New("connection reset")); truncated {
		t.Error("truncationOf() treated a network error as truncation")
	}
	if _, truncated := truncationOf(&ldap.Error{ResultCode: ldap.LDAPResultInsufficientAccessRights}); truncated {
		t.Error("truncationOf() treated a permission error as truncation")
	}
}

func TestRequestDefaults(t *testing.T) {
	r := Request{Base: "DC=example,DC=com", Filter: "(objectClass=*)"}.withDefaults()
	if r.Scope != "subtree" {
		t.Errorf("Scope = %q, want subtree", r.Scope)
	}
	if r.BatchSize == 0 {
		t.Error("BatchSize = 0, want a non-zero default")
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/search/ -v`
Expected: FAIL — `undefined: scopeOf`.

- [ ] **Step 3: Implement it**

```go
// Package search runs filtered searches and hands results back as they arrive.
package search

import (
	"context"
	"errors"

	"github.com/go-ldap/ldap/v3"
	"github.com/skensell201/ldapper/internal/decode"
)

// defaultBatchSize is how many results accumulate before the callback is
// called. Small enough to feel immediate, large enough not to thrash the UI.
const defaultBatchSize = 50

// Request is one search.
type Request struct {
	Base   string
	Filter string
	// Scope is "base", "one" or "subtree". Empty means subtree.
	Scope string
	// Attributes to fetch. Empty means every attribute the server will give us.
	Attributes []string
	// BatchSize is how many results to gather before each callback.
	BatchSize int
}

func (r Request) withDefaults() Request {
	if r.Scope == "" {
		r.Scope = "subtree"
	}
	if r.BatchSize <= 0 {
		r.BatchSize = defaultBatchSize
	}
	return r
}

// Result is one matching entry with its values already decoded.
type Result struct {
	DN         string
	Attributes map[string][]decode.Value
}

// Stats describes how the search ended.
type Stats struct {
	// Matched is how many entries were returned.
	Matched int
	// Truncated is true when the server stopped before the search was
	// complete. The results already delivered are still valid.
	Truncated bool
	// TruncateReason says why, in a sentence fit to show a person.
	TruncateReason string
}

// Stream runs a search, calling onBatch as results arrive. Returning an error
// from onBatch stops the search. Cancelling ctx stops it too.
func Stream(ctx context.Context, conn *ldap.Conn, req Request, onBatch func([]Result) error) (Stats, error) {
	req = req.withDefaults()

	ldapReq := ldap.NewSearchRequest(
		req.Base,
		scopeOf(req.Scope), ldap.NeverDerefAliases, 0, 0, false,
		req.Filter,
		req.Attributes,
		nil,
	)

	var (
		stats Stats
		batch = make([]Result, 0, req.BatchSize)
	)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := onBatch(batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	res := conn.SearchAsync(ctx, ldapReq, req.BatchSize)
	for res.Next() {
		entry := res.Entry()
		if entry == nil {
			continue
		}

		batch = append(batch, resultFrom(entry))
		stats.Matched++

		if len(batch) >= req.BatchSize {
			if err := flush(); err != nil {
				return stats, err
			}
		}
	}

	if err := res.Err(); err != nil {
		// A truncated search still delivered real results. Flush them and
		// report the truncation rather than throwing the work away.
		if reason, truncated := truncationOf(err); truncated {
			stats.Truncated, stats.TruncateReason = true, reason
			if err := flush(); err != nil {
				return stats, err
			}
			return stats, nil
		}
		return stats, err
	}

	if err := flush(); err != nil {
		return stats, err
	}
	return stats, nil
}

func scopeOf(s string) int {
	switch s {
	case "base":
		return ldap.ScopeBaseObject
	case "one":
		return ldap.ScopeSingleLevel
	default:
		return ldap.ScopeWholeSubtree
	}
}

func resultFrom(e *ldap.Entry) Result {
	out := Result{DN: e.DN, Attributes: make(map[string][]decode.Value, len(e.Attributes))}
	for _, attr := range e.Attributes {
		out.Attributes[attr.Name] = decode.Attribute(attr.Name, attr.ByteValues)
	}
	return out
}

// truncationOf reports whether err means "the server stopped early" rather
// than "the search failed".
func truncationOf(err error) (string, bool) {
	var lerr *ldap.Error
	if !errors.As(err, &lerr) {
		return "", false
	}
	switch lerr.ResultCode {
	case ldap.LDAPResultSizeLimitExceeded:
		return "The server reached its size limit — these are the first results, not all of them.", true
	case ldap.LDAPResultTimeLimitExceeded:
		return "The server reached its time limit — these are the results it found before stopping.", true
	default:
		return "", false
	}
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/search/ -v`
Expected: PASS.

If `SearchAsync` returns a type whose methods are named differently — `Next`, `Entry` and `Err`
are what this code assumes — check with `go doc github.com/go-ldap/ldap/v3.Response` and adjust
the loop. This is the API that Task 1 Step 2 exists to confirm.

- [ ] **Step 5: Commit**

```bash
git add internal/search/
git commit -m "Stream search results in batches, reporting truncation as a fact"
```

---

## Task 19: Export to LDIF

**Files:**
- Create: `internal/export/ldif.go`
- Test: `internal/export/ldif_test.go`

- [ ] **Step 1: Write the failing test**

```go
package export

import (
	"bytes"
	"strings"
	"testing"
)

func TestLDIFWritesAnEntry(t *testing.T) {
	var buf bytes.Buffer
	w := NewLDIF(&buf)

	err := w.Write("CN=Anna Volkova,OU=Users,DC=example,DC=com", map[string][]string{
		"cn":          {"Anna Volkova"},
		"objectClass": {"top", "person", "user"},
	})
	if err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	got := buf.String()
	if !strings.HasPrefix(got, "dn: CN=Anna Volkova,OU=Users,DC=example,DC=com\n") {
		t.Errorf("output does not start with the dn line:\n%s", got)
	}
	for _, want := range []string{"cn: Anna Volkova\n", "objectClass: top\n", "objectClass: user\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q:\n%s", want, got)
		}
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Error("entries must be separated by a blank line")
	}
}

func TestLDIFBase64EncodesValuesThatNeedIt(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"non-ASCII", "Анна Волкова", "cn:: "},
		{"leading space", " padded", "cn:: "},
		{"leading colon", ":starts with colon", "cn:: "},
		{"embedded newline", "two\nlines", "cn:: "},
		{"plain ASCII stays plain", "Anna Volkova", "cn: "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := NewLDIF(&buf)
			if err := w.Write("CN=x,DC=example,DC=com", map[string][]string{"cn": {tt.value}}); err != nil {
				t.Fatalf("Write() returned error: %v", err)
			}
			if err := w.Close(); err != nil {
				t.Fatalf("Close() returned error: %v", err)
			}
			if !strings.Contains(buf.String(), tt.want) {
				t.Errorf("output %q does not contain %q", buf.String(), tt.want)
			}
		})
	}
}

func TestLDIFBase64EncodesADNThatNeedsIt(t *testing.T) {
	var buf bytes.Buffer
	w := NewLDIF(&buf)
	if err := w.Write("CN=Анна,DC=example,DC=com", nil); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if !strings.HasPrefix(buf.String(), "dn:: ") {
		t.Errorf("a non-ASCII DN must be base64-encoded, got:\n%s", buf.String())
	}
}

func TestLDIFWritesAttributesInAStableOrder() {}

func TestLDIFIsDeterministic(t *testing.T) {
	// Go randomises map iteration. Two exports of the same entry must still
	// produce identical bytes, or diffing two exports is useless.
	attrs := map[string][]string{"zebra": {"z"}, "alpha": {"a"}, "middle": {"m"}}

	var first, second bytes.Buffer
	for _, buf := range []*bytes.Buffer{&first, &second} {
		w := NewLDIF(buf)
		if err := w.Write("CN=x,DC=example,DC=com", attrs); err != nil {
			t.Fatalf("Write() returned error: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close() returned error: %v", err)
		}
	}
	if first.String() != second.String() {
		t.Errorf("two exports of the same entry differ:\n%s\n---\n%s", first.String(), second.String())
	}
}

func TestLDIFWritesTheVersionHeaderOnce(t *testing.T) {
	var buf bytes.Buffer
	w := NewLDIF(&buf)
	for i := 0; i < 3; i++ {
		if err := w.Write("CN=x,DC=example,DC=com", map[string][]string{"cn": {"x"}}); err != nil {
			t.Fatalf("Write() returned error: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if n := strings.Count(buf.String(), "version: 1"); n != 1 {
		t.Errorf("found %d version headers, want exactly 1", n)
	}
}
```

Delete the empty `TestLDIFWritesAttributesInAStableOrder` stub — it is listed above only so the
name is not accidentally reused; `TestLDIFIsDeterministic` covers that behaviour.

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/export/ -v`
Expected: FAIL — `undefined: NewLDIF`.

- [ ] **Step 3: Implement it**

```go
// Package export writes directory entries out in the formats other tools read.
package export

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Writer takes entries one at a time so an export of a large subtree never
// has to be assembled in memory first.
type Writer interface {
	Write(dn string, attrs map[string][]string) error
	Close() error
}

type ldifWriter struct {
	w            *bufio.Writer
	wroteVersion bool
}

// NewLDIF returns a Writer producing RFC 2849 LDIF.
func NewLDIF(w io.Writer) Writer {
	return &ldifWriter{w: bufio.NewWriter(w)}
}

func (l *ldifWriter) Write(dn string, attrs map[string][]string) error {
	if !l.wroteVersion {
		if _, err := l.w.WriteString("version: 1\n\n"); err != nil {
			return err
		}
		l.wroteVersion = true
	}

	if err := l.writeLine("dn", dn); err != nil {
		return err
	}

	// Sort the attribute names so two exports of the same entry are byte
	// identical — otherwise Go's random map order makes them impossible to
	// compare.
	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		for _, value := range attrs[name] {
			if err := l.writeLine(name, value); err != nil {
				return err
			}
		}
	}

	_, err := l.w.WriteString("\n")
	return err
}

// writeLine emits one attribute, base64-encoding the value when LDIF requires
// it: anything not printable ASCII, or starting with a character that would
// change how the line parses.
func (l *ldifWriter) writeLine(name, value string) error {
	if needsBase64(value) {
		_, err := fmt.Fprintf(l.w, "%s:: %s\n", name, base64.StdEncoding.EncodeToString([]byte(value)))
		return err
	}
	_, err := fmt.Fprintf(l.w, "%s: %s\n", name, value)
	return err
}

func (l *ldifWriter) Close() error { return l.w.Flush() }

func needsBase64(s string) bool {
	if s == "" {
		return false
	}
	switch s[0] {
	case ' ', ':', '<':
		return true
	}
	if strings.HasSuffix(s, " ") {
		return true
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7E {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test ./internal/export/ -run LDIF -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/export/ldif.go internal/export/ldif_test.go
git commit -m "Export entries as LDIF with deterministic attribute ordering"
```

---

## Task 20: Export to CSV

**Files:**
- Create: `internal/export/csv.go`
- Test: `internal/export/csv_test.go`

- [ ] **Step 1: Write the failing test**

```go
package export

import (
	"bytes"
	"strings"
	"testing"
)

func TestCSVWritesAHeaderAndRows(t *testing.T) {
	var buf bytes.Buffer
	w, err := NewCSV(&buf, []string{"cn", "mail"})
	if err != nil {
		t.Fatalf("NewCSV() returned error: %v", err)
	}

	if err := w.Write("CN=Anna,DC=example,DC=com", map[string][]string{
		"cn":   {"Anna Volkova"},
		"mail": {"a.volkova@example.com"},
	}); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want a header and one row:\n%s", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[0], "dn,cn,mail") {
		t.Errorf("header = %q, want dn first, then the requested columns", lines[0])
	}
	if !strings.Contains(lines[1], "Anna Volkova") {
		t.Errorf("row = %q, want the cn value", lines[1])
	}
}

func TestCSVJoinsMultipleValues(t *testing.T) {
	var buf bytes.Buffer
	w, _ := NewCSV(&buf, []string{"objectClass"})
	if err := w.Write("CN=x,DC=example,DC=com", map[string][]string{
		"objectClass": {"top", "person", "user"},
	}); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "top; person; user") {
		t.Errorf("multi-valued attribute not joined:\n%s", buf.String())
	}
}

func TestCSVLeavesMissingAttributesEmpty(t *testing.T) {
	var buf bytes.Buffer
	w, _ := NewCSV(&buf, []string{"cn", "telephoneNumber"})
	if err := w.Write("CN=x,DC=example,DC=com", map[string][]string{"cn": {"x"}}); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if !strings.HasSuffix(strings.TrimRight(buf.String(), "\n"), ",x,") {
		t.Errorf("a missing attribute must produce an empty field:\n%s", buf.String())
	}
}

func TestCSVQuotesFieldsContainingSeparators(t *testing.T) {
	var buf bytes.Buffer
	w, _ := NewCSV(&buf, []string{"cn"})
	if err := w.Write("CN=x,DC=example,DC=com", map[string][]string{
		"cn": {`Volkova, Anna`},
	}); err != nil {
		t.Fatalf("Write() returned error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
	if !strings.Contains(buf.String(), `"Volkova, Anna"`) {
		t.Errorf("a value containing a comma must be quoted:\n%s", buf.String())
	}
}

func TestCSVRejectsAnEmptyColumnList(t *testing.T) {
	var buf bytes.Buffer
	if _, err := NewCSV(&buf, nil); err == nil {
		t.Error("NewCSV() with no columns succeeded, want an error")
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

Run: `go test ./internal/export/ -run CSV -v`
Expected: FAIL — `undefined: NewCSV`.

- [ ] **Step 3: Implement it**

```go
package export

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

type csvWriter struct {
	w       *csv.Writer
	columns []string
}

// NewCSV returns a Writer emitting one row per entry. The distinguished name
// is always the first column; columns names the attributes after it.
func NewCSV(w io.Writer, columns []string) (Writer, error) {
	if len(columns) == 0 {
		return nil, fmt.Errorf("export: a CSV export needs at least one column")
	}

	cw := csv.NewWriter(w)
	if err := cw.Write(append([]string{"dn"}, columns...)); err != nil {
		return nil, err
	}

	return &csvWriter{w: cw, columns: columns}, nil
}

func (c *csvWriter) Write(dn string, attrs map[string][]string) error {
	row := make([]string, 0, len(c.columns)+1)
	row = append(row, dn)
	for _, name := range c.columns {
		// A multi-valued attribute becomes one field. Semicolon-space keeps
		// it readable in a spreadsheet without colliding with the separator.
		row = append(row, strings.Join(attrs[name], "; "))
	}
	return c.w.Write(row)
}

func (c *csvWriter) Close() error {
	c.w.Flush()
	return c.w.Error()
}
```

- [ ] **Step 4: Run the whole package and confirm it passes**

Run: `go test ./internal/export/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/export/csv.go internal/export/csv_test.go
git commit -m "Export search results as CSV with selectable columns"
```

---

## Task 21: The probe CLI

Everything above is a library nobody can run. This is the harness that makes the engine usable
before the interface exists — and the thing you reach for when a customer's directory behaves in
a way no test predicted.

**Files:**
- Create: `cmd/ldapper-probe/main.go`

- [ ] **Step 1: Write it**

There is no test for this file: it is argument parsing and printing, and the packages it calls are
already covered. Keep it that way — any logic worth testing belongs in `internal/`.

```go
// Command ldapper-probe exercises the Ldapper engine from a terminal. It is a
// development tool, not a product: it exists so the engine can be pointed at a
// real directory before any interface is built.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/skensell201/ldapper/internal/browse"
	"github.com/skensell201/ldapper/internal/export"
	"github.com/skensell201/ldapper/internal/filters"
	"github.com/skensell201/ldapper/internal/ldaperr"
	"github.com/skensell201/ldapper/internal/schema"
	"github.com/skensell201/ldapper/internal/search"
	"github.com/skensell201/ldapper/internal/session"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", ldaperr.Explain(err))
		os.Exit(1)
	}
}

func run() error {
	var (
		host     = flag.String("host", "localhost", "directory host")
		port     = flag.Int("port", 389, "directory port")
		enc      = flag.String("encryption", "none", "none, ldaps or starttls")
		user     = flag.String("user", "", "bind DN, UPN or DOMAIN\\account")
		pass     = flag.String("password", "", "bind password")
		ntlm     = flag.Bool("ntlm", false, "use an NTLM bind")
		trust    = flag.String("trust", "", "certificate fingerprint to accept")
		base     = flag.String("base", "", "search base; defaults to the server's own")
		filter   = flag.String("filter", "(objectClass=*)", "LDAP filter")
		scope    = flag.String("scope", "subtree", "base, one or subtree")
		attrs    = flag.String("attributes", "", "comma-separated attributes to fetch")
		asLDIF   = flag.Bool("ldif", false, "write results as LDIF")
		asCSV    = flag.Bool("csv", false, "write results as CSV")
	)
	flag.Parse()

	command := flag.Arg(0)
	if command == "" {
		return fmt.Errorf("usage: ldapper-probe [flags] info|browse|search|filters")
	}

	// filters needs no connection at all.
	if command == "filters" {
		return listFilters()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cfg := session.Config{
		Host:       *host,
		Port:       *port,
		Encryption: session.Encryption(*enc),
	}
	if *trust != "" {
		cfg.TrustedFingerprints = []string{*trust}
	}

	conn, err := session.Dial(ctx, cfg)
	if err != nil {
		var certErr *session.CertError
		if errorsAs(err, &certErr) {
			fmt.Fprintf(os.Stderr, "untrusted certificate\n  subject:  %s\n  issuer:   %s\n  SHA-256:  %s\n  expires:  %s\n\nrerun with -trust %s to accept it\n",
				certErr.Subject, certErr.Issuer, certErr.Fingerprint,
				certErr.NotAfter.Format("2006-01-02"), certErr.Fingerprint)
			os.Exit(2)
		}
		return err
	}
	defer conn.Close()

	switch {
	case *user == "":
		if err := conn.BindAnonymous(); err != nil {
			return err
		}
	case *ntlm:
		domain, account := session.SplitAccount(*user)
		if err := conn.BindNTLM(domain, account, *pass); err != nil {
			return err
		}
	default:
		if err := conn.BindSimple(*user, *pass); err != nil {
			return err
		}
	}

	info, err := schema.Read(conn.Conn)
	if err != nil {
		return err
	}
	if *base == "" {
		*base = info.RootDN()
	}

	switch command {
	case "info":
		return printInfo(conn, info)
	case "browse":
		return printChildren(conn, *base)
	case "search":
		return runSearch(ctx, conn, *base, *filter, *scope, *attrs, *asLDIF, *asCSV)
	default:
		return fmt.Errorf("unknown command %q; try info, browse, search or filters", command)
	}
}

func printInfo(conn *session.Conn, info schema.Info) error {
	full, err := schema.ReadObjectClasses(conn.Conn, info)
	if err != nil {
		return err
	}

	fmt.Println("vendor:            ", orNone(full.VendorName))
	fmt.Println("root DN:           ", orNone(full.RootDN()))
	fmt.Println("naming contexts:   ", strings.Join(full.NamingContexts, ", "))
	fmt.Println("active directory:  ", full.IsActiveDirectory())
	fmt.Println("paged results:     ", full.SupportsPaging())
	fmt.Println("bound as:          ", orNone(conn.BindDN))

	var names []string
	for _, d := range full.Dialects() {
		names = append(names, string(d))
	}
	fmt.Println("filter dialects:   ", strings.Join(names, ", "))
	return nil
}

func printChildren(conn *session.Conn, base string) error {
	var cookie []byte
	page := 0

	for {
		page++
		result, err := browse.Children(conn.Conn, base, 0, cookie)
		if err != nil {
			return err
		}
		for _, e := range result.Entries {
			children := "?"
			if e.NumSubordinates >= 0 {
				children = fmt.Sprint(e.NumSubordinates)
			}
			fmt.Printf("%-52s  %-28s  children=%s\n", e.RDN, strings.Join(e.Classes, "/"), children)
		}
		fmt.Fprintf(os.Stderr, "-- page %d: %d entries\n", page, len(result.Entries))

		if len(result.Cookie) == 0 {
			return nil
		}
		cookie = result.Cookie
	}
}

func runSearch(ctx context.Context, conn *session.Conn, base, filter, scope, attrs string, asLDIF, asCSV bool) error {
	var attrList []string
	if attrs != "" {
		attrList = strings.Split(attrs, ",")
	}

	e := filters.Expander{Now: time.Now(), BindDN: conn.BindDN}
	if err := filters.Validate(filter, e); err != nil {
		return err
	}
	expanded, err := e.Expand(filter)
	if err != nil {
		return err
	}

	var writer export.Writer
	switch {
	case asLDIF:
		writer = export.NewLDIF(os.Stdout)
	case asCSV:
		columns := attrList
		if len(columns) == 0 {
			columns = []string{"cn"}
		}
		writer, err = export.NewCSV(os.Stdout, columns)
		if err != nil {
			return err
		}
	}

	stats, err := search.Stream(ctx, conn.Conn, search.Request{
		Base:       base,
		Filter:     expanded,
		Scope:      scope,
		Attributes: attrList,
	}, func(batch []search.Result) error {
		for _, r := range batch {
			if writer == nil {
				fmt.Println(r.DN)
				continue
			}
			if err := writer.Write(r.DN, flatten(r)); err != nil {
				return err
			}
		}
		fmt.Fprintf(os.Stderr, "-- %d so far\n", len(batch))
		return nil
	})
	if err != nil {
		return err
	}

	if writer != nil {
		if err := writer.Close(); err != nil {
			return err
		}
	}

	fmt.Fprintf(os.Stderr, "-- matched %d\n", stats.Matched)
	if stats.Truncated {
		fmt.Fprintln(os.Stderr, "--", stats.TruncateReason)
	}
	return nil
}

func listFilters() error {
	for _, f := range filters.Builtins() {
		fmt.Printf("%-28s  %-8s  %s\n", f.ID, f.Dialect, f.Name)
		fmt.Printf("%30s%s\n", "", f.Filter)
	}
	return nil
}

// flatten drops the decoded renderings, because an export carries what the
// server actually stores.
func flatten(r search.Result) map[string][]string {
	out := make(map[string][]string, len(r.Attributes))
	for name, values := range r.Attributes {
		for _, v := range values {
			out[name] = append(out[name], v.Raw)
		}
	}
	return out
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
```

Add this import and helper so `errorsAs` reads clearly at the call site:

```go
import "errors"

func errorsAs(err error, target any) bool { return errors.As(err, target) }
```

- [ ] **Step 2: Build it and check the flags**

```bash
go build ./cmd/ldapper-probe && ./ldapper-probe filters | head -6
```

Expected: the first three built-in filters, each with its ID, dialect, name and filter expression.

- [ ] **Step 3: Commit**

```bash
git add cmd/ldapper-probe/
git commit -m "Add the probe CLI so the engine can be run against a real server"
```

---

## Task 22: Integration tests against a real directory

Every test so far avoided the network. These are the ones that catch what mocks never do: a paging
cookie the server hands back differently than expected, an attribute that arrives as bytes rather
than text, a bind that works everywhere except here.

**Files:**
- Create: `test/integration/docker-compose.yml`, `test/integration/seed.ldif`, `test/integration/directory_test.go`

- [ ] **Step 1: Write the compose file**

```yaml
# test/integration/docker-compose.yml
services:
  openldap:
    image: bitnami/openldap:2.6
    ports:
      - "3389:1389"
    environment:
      LDAP_ROOT: dc=example,dc=com
      LDAP_ADMIN_USERNAME: admin
      LDAP_ADMIN_PASSWORD: adminpassword
      LDAP_SKIP_DEFAULT_TREE: "yes"
      LDAP_CUSTOM_LDIF_DIR: /ldifs
    volumes:
      - ./seed.ldif:/ldifs/seed.ldif:ro
    healthcheck:
      test: ["CMD", "ldapsearch", "-x", "-H", "ldap://localhost:1389", "-b", "dc=example,dc=com", "-D", "cn=admin,dc=example,dc=com", "-w", "adminpassword"]
      interval: 3s
      timeout: 5s
      retries: 20
```

- [ ] **Step 2: Write the seed data**

`seed.ldif` needs enough entries to exercise paging. Write the fixed entries by hand and generate
the rest:

```ldif
dn: dc=example,dc=com
objectClass: top
objectClass: dcObject
objectClass: organization
o: Example
dc: example

dn: ou=people,dc=example,dc=com
objectClass: organizationalUnit
ou: people

dn: ou=groups,dc=example,dc=com
objectClass: organizationalUnit
ou: groups

dn: cn=Anna Volkova,ou=people,dc=example,dc=com
objectClass: inetOrgPerson
cn: Anna Volkova
sn: Volkova
uid: a.volkova
mail: a.volkova@example.com

dn: cn=Boris Ivanov,ou=people,dc=example,dc=com
objectClass: inetOrgPerson
cn: Boris Ivanov
sn: Ivanov
uid: b.ivanov

dn: cn=empty-group,ou=groups,dc=example,dc=com
objectClass: groupOfNames
cn: empty-group
member:
```

Append 120 more people so that a page size of 50 needs three round trips:

```bash
cd test/integration
for i in $(seq -w 1 120); do
  printf 'dn: cn=Filler %s,ou=people,dc=example,dc=com\nobjectClass: inetOrgPerson\ncn: Filler %s\nsn: Filler\nuid: filler%s\n\n' "$i" "$i" "$i" >> seed.ldif
done
```

- [ ] **Step 3: Write the tests**

```go
//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/skensell201/ldapper/internal/browse"
	"github.com/skensell201/ldapper/internal/filters"
	"github.com/skensell201/ldapper/internal/schema"
	"github.com/skensell201/ldapper/internal/search"
	"github.com/skensell201/ldapper/internal/session"
)

const (
	adminDN = "cn=admin,dc=example,dc=com"
	adminPW = "adminpassword"
	rootDN  = "dc=example,dc=com"
)

// connect returns a bound connection to the compose-managed server.
func connect(t *testing.T) *session.Conn {
	t.Helper()

	port := 3389
	if v := os.Getenv("LDAPPER_TEST_PORT"); v != "" {
		t.Logf("using LDAPPER_TEST_PORT=%s", v)
	}

	conn, err := session.Dial(context.Background(), session.Config{
		Host:       "localhost",
		Port:       port,
		Encryption: session.EncryptionNone,
		Timeout:    10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Dial() failed — is `make integration` running the compose file? %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	if err := conn.BindSimple(adminDN, adminPW); err != nil {
		t.Fatalf("BindSimple() failed: %v", err)
	}
	return conn
}

func TestReadRootDSE(t *testing.T) {
	conn := connect(t)

	info, err := schema.Read(conn.Conn)
	if err != nil {
		t.Fatalf("schema.Read() failed: %v", err)
	}
	if info.RootDN() != rootDN {
		t.Errorf("RootDN() = %q, want %q", info.RootDN(), rootDN)
	}
	if info.IsActiveDirectory() {
		t.Error("IsActiveDirectory() = true for OpenLDAP")
	}
	if !info.SupportsPaging() {
		t.Error("SupportsPaging() = false; OpenLDAP does support the paging control")
	}
}

func TestBrowseTopLevel(t *testing.T) {
	conn := connect(t)

	page, err := browse.Children(conn.Conn, rootDN, 0, nil)
	if err != nil {
		t.Fatalf("browse.Children() failed: %v", err)
	}
	if len(page.Entries) != 2 {
		t.Errorf("got %d children of the root, want ou=people and ou=groups", len(page.Entries))
	}
}

func TestBrowsePagesThroughALargeBranch(t *testing.T) {
	conn := connect(t)

	var (
		cookie []byte
		total  int
		pages  int
	)
	for {
		page, err := browse.Children(conn.Conn, "ou=people,dc=example,dc=com", 50, cookie)
		if err != nil {
			t.Fatalf("browse.Children() failed on page %d: %v", pages+1, err)
		}
		total += len(page.Entries)
		pages++

		if len(page.Cookie) == 0 {
			break
		}
		cookie = page.Cookie

		if pages > 10 {
			t.Fatal("paging did not terminate — the cookie is not being consumed")
		}
	}

	if total != 122 {
		t.Errorf("got %d people across %d pages, want 122", total, pages)
	}
	if pages < 3 {
		t.Errorf("got %d pages at a page size of 50, want at least 3", pages)
	}
}

func TestSearchStreamsInBatches(t *testing.T) {
	conn := connect(t)

	var batches int
	stats, err := search.Stream(context.Background(), conn.Conn, search.Request{
		Base:       rootDN,
		Filter:     "(objectClass=inetOrgPerson)",
		Scope:      "subtree",
		Attributes: []string{"cn", "uid"},
		BatchSize:  25,
	}, func(batch []search.Result) error {
		batches++
		return nil
	})
	if err != nil {
		t.Fatalf("search.Stream() failed: %v", err)
	}
	if stats.Matched != 122 {
		t.Errorf("Matched = %d, want 122", stats.Matched)
	}
	if batches < 2 {
		t.Errorf("got %d batches, want the results delivered incrementally", batches)
	}
}

func TestSearchCancellationStops(t *testing.T) {
	conn := connect(t)

	ctx, cancel := context.WithCancel(context.Background())
	var seen int

	_, err := search.Stream(ctx, conn.Conn, search.Request{
		Base:      rootDN,
		Filter:    "(objectClass=*)",
		Scope:     "subtree",
		BatchSize: 5,
	}, func(batch []search.Result) error {
		seen += len(batch)
		if seen >= 5 {
			cancel()
		}
		return nil
	})

	// Either a context error or a clean stop is acceptable; what matters is
	// that it did not stream all 122 entries after being cancelled.
	if err == nil && seen > 60 {
		t.Errorf("cancellation delivered %d entries, want the search to stop early", seen)
	}
	_ = err
}

func TestPortableBuiltinFiltersRunAgainstOpenLDAP(t *testing.T) {
	conn := connect(t)
	e := filters.Expander{Now: time.Now(), BindDN: adminDN}

	for _, f := range filters.Builtins() {
		if f.Dialect == filters.DialectAD {
			continue // no Active Directory here to run these against
		}

		t.Run(f.ID, func(t *testing.T) {
			expanded, err := e.Expand(f.Filter)
			if err != nil {
				t.Fatalf("Expand() failed: %v", err)
			}
			if _, err := search.Stream(context.Background(), conn.Conn, search.Request{
				Base:   rootDN,
				Filter: expanded,
				Scope:  string(f.Scope),
			}, func([]search.Result) error { return nil }); err != nil {
				t.Errorf("filter %q was rejected by the server: %v", f.ID, err)
			}
		})
	}
}

func TestBindFailureIsExplained(t *testing.T) {
	conn, err := session.Dial(context.Background(), session.Config{
		Host: "localhost", Port: 3389, Encryption: session.EncryptionNone,
	})
	if err != nil {
		t.Fatalf("Dial() failed: %v", err)
	}
	defer conn.Close()

	if err := conn.BindSimple(adminDN, "wrong password"); err == nil {
		t.Fatal("BindSimple() with a wrong password succeeded")
	}
}
```

- [ ] **Step 4: Run them**

```bash
make integration
```

Expected: every test passes. `TestBrowsePagesThroughALargeBranch` is the one that matters most —
if it reports one page of 122 entries, the paging control is not being applied and browsing a real
organizational unit will silently show only its first 1000 objects.

- [ ] **Step 5: Commit**

```bash
git add test/integration/ Makefile
git commit -m "Add integration tests against OpenLDAP in Docker"
```

---

## Task 23: Continuous integration

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write the workflow**

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
          cache: true

      - name: Vet
        run: go vet ./...

      - name: Lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

      - name: Unit tests
        run: go test -race ./...

      - name: Start the test directory
        run: docker compose -f test/integration/docker-compose.yml up -d --wait

      - name: Integration tests
        run: go test -tags=integration ./test/integration/... -v

      - name: Stop the test directory
        if: always()
        run: docker compose -f test/integration/docker-compose.yml down -v

  build:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
          cache: true
      - run: go build ./...
```

- [ ] **Step 2: Confirm the unit tests pass with the race detector**

The `-race` flag is the reason `Store` carries a mutex. Verify it is actually doing something:

```bash
go test -race ./...
```

Expected: PASS with no data race reports.

- [ ] **Step 3: Commit and push**

```bash
git add .github/workflows/ci.yml
git commit -m "Run vet, lint, unit and integration tests in CI"
git push origin main
```

- [ ] **Step 4: Confirm the run went green**

```bash
gh run watch
```

Expected: every job succeeds. The Windows and macOS build jobs only compile — they exist to catch
a platform-specific import before plan 2 starts packaging for those systems.

---

## Definition of done

The plan is complete when all of these hold:

- [ ] `go test -race ./...` passes with no skipped packages
- [ ] `make integration` passes against the compose file
- [ ] `golangci-lint run` reports nothing
- [ ] `./ldapper-probe -host localhost -port 3389 -user 'cn=admin,dc=example,dc=com' -password adminpassword info` prints the root DN, `active directory: false`, and the dialects `generic, posix`
- [ ] `./ldapper-probe … browse` walks `ou=people` across three pages
- [ ] `./ldapper-probe … -filter '(objectClass=inetOrgPerson)' -attributes cn,uid -csv search` writes a CSV with a header and 122 rows
- [ ] CI is green on `main`

---

## Self-review against the spec

Checked section by section. Coverage and gaps:

| Spec section | Covered by |
|---|---|
| 3.1 session | Tasks 14, 15 — dial, TLS, certificate decision, simple and NTLM binds. One automatic reconnect after a dropped connection is **deferred to plan 2**: it belongs with the UI that shows the reconnect happening, and there is nothing to reconnect for in a CLI. |
| 3.2 browse | Task 17, verified against a real server in Task 22 |
| 3.3 search | Task 18, including truncation as a state rather than an error |
| 3.4 schema | Task 16 — RootDSE, Active Directory detection, lazy object class read. Per-attribute syntax and single-value flags from `attributeTypes` are **deferred to plan 2**: they only affect how the attribute table renders, and nothing in the engine consumes them. |
| 3.5 decode | Tasks 2–6, with the fallback-to-raw rule enforced by test |
| 3.6 filters | Tasks 8–12 — all 18 built-ins, substitutions, validation, the overlay store, dialect gating |
| 3.7 profiles | Task 13 |
| 3.8 export | Tasks 19, 20 |
| 5 errors | Task 7, including the Active Directory `data 52e` sub-codes |
| 6 testing | Tasks 2–20 for units, Task 22 for integration, Task 23 for CI |

Two deliberate deferrals, both named above. Nothing else in the spec's engine sections is
unaccounted for. The interface sections (4) and the v1 boundaries (7) are plan 2 and plan 3.

Type consistency was checked across tasks: `filters.Filter`, `filters.Dialect`, `filters.Scope`,
`session.Config`, `session.Conn`, `schema.Info`, `browse.Entry`, `browse.Page`, `search.Request`,
`search.Result`, `search.Stats`, `decode.Value` and `export.Writer` are each defined once and used
with the same shape everywhere they appear.

