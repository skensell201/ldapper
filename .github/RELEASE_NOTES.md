Fixes from the first run against a real domain controller.

## Active Directory is recognised again

This is the one that mattered. `1.2.840.113556.1.4.800` is a *capability*, and
Active Directory publishes it in `supportedCapabilities` — never in
`supportedControl`, which is where Ldapper looked for it.

Every domain controller was therefore reported as plain LDAP. The status bar
said so, and all twelve Active Directory filters sat greyed out with a message
explaining that the server did not support them. It did.

If you connected to a domain and wondered why the filters were unavailable,
that was why.

## The bound identity fits now

`CN=SpaceReader,OU=Service Accounts,DC=da,DC=lan` ran off the end of the
connection pill and was cut mid-word. Only the leftmost value is shown —
`SpaceReader` — with the whole name in the tooltip. A comma inside a name is
unescaped there too.

## The status bar agrees with the chrome

They described the connection separately and could drift apart. They share an
indicator and a colour now: white when connected, coral when something went
wrong. An unencrypted connection is called out rather than stated flatly.

## Run and Delete look like controls

Both read as labels that happened to be clickable. Run is outlined as the
thing you came to the editor to press. Delete shows its intent on approach
instead of sitting permanently red next to Save.

## Verified

315 Go tests, 33 frontend tests, 37 integration tests. The Active Directory
detection has tests covering a domain controller's real capability list, a
server that republishes the marker among its controls, and OpenLDAP, which
publishes neither and must not be mistaken for a domain.
