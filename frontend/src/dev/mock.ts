/** A stand-in for the Go side, used only by the Vite dev server.
 *
 *  The window injects `window.go` before the bundle runs. When it is absent —
 *  which only happens under `npm run dev` — these take over, so the interface
 *  can be worked on without building the application, and so its screenshots
 *  can be taken in a browser.
 *
 *  The data mirrors dev/directory: the same tree, the same awkward entries.
 */

const people = [
  "Anna Volkova", "Boris Ivanov", "Daria Orlova", "Egor Titov", "Irina Belova",
  "Kirill Zaytsev", "Marina Rud", "Nikita Frolov", "Olga Sokolova", "Pavel Morozov",
];

const ROOT = "dc=example,dc=com";

function node(dn: string, label: string, icon: string, children = -1) {
  return { dn, label, rdn: label, icon, childCount: children, hasChildren: children !== 0 };
}

const tree: Record<string, ReturnType<typeof node>[]> = {
  [ROOT]: [
    node(`ou=corp,${ROOT}`, "corp", "ou"),
    node(`ou=computers,${ROOT}`, "computers", "ou"),
    node(`ou=system,${ROOT}`, "system", "container"),
  ],
  [`ou=corp,${ROOT}`]: [
    node(`ou=people,ou=corp,${ROOT}`, "people", "ou"),
    node(`ou=groups,ou=corp,${ROOT}`, "groups", "ou"),
    node(`ou=service accounts,ou=corp,${ROOT}`, "service accounts", "ou"),
    node(`ou=departments,ou=corp,${ROOT}`, "departments", "ou"),
  ],
  [`ou=people,ou=corp,${ROOT}`]: [
    node(`cn=Volkova\\2C Anna,ou=people,ou=corp,${ROOT}`, "Volkova, Anna", "user", 0),
    node(`cn=Анна Волкова,ou=people,ou=corp,${ROOT}`, "Анна Волкова", "user", 0),
    ...people.map((p, i) => node(`cn=${p} ${i + 1},ou=people,ou=corp,${ROOT}`, `${p} ${i + 1}`, "user", 0)),
  ],
  [`ou=groups,ou=corp,${ROOT}`]: [
    node(`cn=all staff,ou=groups,ou=corp,${ROOT}`, "all staff", "group", 0),
    node(`cn=vpn users,ou=groups,ou=corp,${ROOT}`, "vpn users", "group", 0),
    node(`cn=administrators,ou=groups,ou=corp,${ROOT}`, "administrators", "group", 0),
    node(`cn=empty group,ou=groups,ou=corp,${ROOT}`, "empty group", "group", 0),
  ],
};

const detail = {
  dn: `cn=Volkova\\2C Anna,ou=people,ou=corp,${ROOT}`,
  rdn: "cn=Volkova\\2C Anna",
  icon: "user",
  rows: [
    { name: "cn", values: [{ raw: "Volkova, Anna" }] },
    { name: "mail", values: [{ raw: "a.volkova@example.com" }, { raw: "anna@example.com" }] },
    {
      name: "objectClass",
      values: [{ raw: "top" }, { raw: "person" }, { raw: "organizationalPerson" }, { raw: "inetOrgPerson" }],
    },
    {
      name: "objectSid",
      values: [{
        raw: "AQUAAAAAAAUVAAAAx1HaZLwzNM8UAe0BUAQAAA==",
        decoded: ["S-1-5-21-3638186439-3476304828-32309524-1104"],
      }],
    },
    {
      name: "pwdLastSet",
      values: [{ raw: "133894094610000000", decoded: ["2025-04-18 00:24:21 UTC"] }],
    },
    {
      name: "userAccountControl",
      values: [{ raw: "66048", decoded: ["NORMAL_ACCOUNT", "DONT_EXPIRE_PASSWORD"] }],
    },
    {
      name: "accountExpires",
      values: [{ raw: "9223372036854775807", decoded: ["never"] }],
    },
    { name: "sn", values: [{ raw: "Volkova" }] },
    { name: "telephoneNumber", values: [{ raw: "+49 30 123456" }] },
    { name: "title", values: [{ raw: "Systems Engineer" }] },
    {
      name: "whenCreated",
      values: [{ raw: "20230914081233.0Z", decoded: ["2023-09-14 08:12:33 UTC"] }],
    },
  ],
};

const state = {
  profileId: "demo",
  connected: true,
  host: "dc01.corp.example.com",
  boundAs: "CN=SpaceReader,OU=Service Accounts,DC=corp,DC=example,DC=com",
  rootDN: ROOT,
  isActiveDirectory: true,
  supportsPaging: true,
  dialects: ["generic", "ad"],
  encryption: "ldaps",
};

const builtins: [string, string, string, string][] = [
  ["ad-disabled-accounts", "Disabled accounts", "ad", "(&(objectCategory=person)(objectClass=user)(userAccountControl:1.2.840.113556.1.4.803:=2))"],
  ["ad-locked-out", "Locked out right now", "ad", "(&(objectCategory=person)(objectClass=user)(lockoutTime>=1))"],
  ["ad-password-never-expires", "Password never expires", "ad", "(&(objectCategory=person)(objectClass=user)(userAccountControl:1.2.840.113556.1.4.803:=65536))"],
  ["ad-must-change-password", "Must change at next logon", "ad", "(&(objectCategory=person)(objectClass=user)(pwdLastSet=0))"],
  ["ad-stale-users-90d", "Stale users · 90 days", "ad", "(&(objectCategory=person)(objectClass=user)(!(userAccountControl:1.2.840.113556.1.4.803:=2))(lastLogonTimestamp<={{now-90d:filetime}}))"],
  ["ad-stale-computers-60d", "Stale computers · 60 days", "ad", "(&(objectCategory=computer)(lastLogonTimestamp<={{now-60d:filetime}}))"],
  ["ad-privileged-accounts", "Privileged accounts", "ad", "(&(objectCategory=person)(objectClass=user)(adminCount=1))"],
  ["ad-accounts-with-spn", "Accounts with an SPN", "ad", "(&(objectCategory=person)(objectClass=user)(servicePrincipalName=*))"],
  ["ad-no-preauth", "Kerberos pre-auth disabled", "ad", "(&(objectCategory=person)(objectClass=user)(userAccountControl:1.2.840.113556.1.4.803:=4194304))"],
  ["ad-unconstrained-delegation", "Unconstrained delegation", "ad", "(&(objectCategory=computer)(userAccountControl:1.2.840.113556.1.4.803:=524288))"],
  ["ad-domain-controllers", "Domain controllers", "ad", "(&(objectCategory=computer)(userAccountControl:1.2.840.113556.1.4.803:=8192))"],
  ["ad-group-policy-objects", "Group policy objects", "ad", "(objectClass=groupPolicyContainer)"],
  ["all-people", "All people", "generic", "(|(objectClass=person)(objectClass=inetOrgPerson))"],
  ["all-groups", "All groups", "generic", "(|(objectClass=group)(objectClass=groupOfNames)(objectClass=groupOfUniqueNames)(objectClass=posixGroup))"],
  ["all-organizational-units", "All organizational units", "generic", "(objectClass=organizationalUnit)"],
  ["empty-groups", "Empty groups", "generic", "(|(&(|(objectClass=group)(objectClass=groupOfNames))(!(member=*)))(&(objectClass=posixGroup)(!(memberUid=*))))"],
  ["people-without-email", "People without email", "generic", "(&(objectClass=person)(!(mail=*)))"],
  ["posix-accounts", "POSIX accounts", "posix", "(objectClass=posixAccount)"],
];

const titles = ["Systems Engineer", "Accountant", "Support Analyst", "Team Lead", "Controller"];

export const mock = {
  Build: async () => ({ version: "dev", commit: "", modified: false }),
  ListProfiles: async () => [{
    id: "demo", name: "Example Corporation", host: state.host, port: 636,
    encryption: "ldaps", bindMethod: "simple", domain: "",
    username: state.boundAs, connected: true,
  }],
  Connect: async () => ({ trusted: true, state }),
  Disconnect: async () => {},
  TrustCertificate: async () => "",
  Children: async (_id: string, dn: string) => ({ nodes: tree[dn] ?? [], cookie: "", error: "" }),
  Entry: async () => ({ detail, error: "" }),
  SaveProfile: async () => "",
  DeleteProfile: async () => "",
  ListFilters: async () => builtins.map(([id, name, dialect, filter]) => ({
    id, name, dialect, filter,
    description: "One of the filters Ldapper ships with.",
    scope: "subtree", base: "", columns: ["cn", "sAMAccountName", "distinguishedName"],
    builtIn: true, modified: id === "ad-stale-users-90d",
    supported: dialect !== "posix",
    reason: dialect === "posix" ? "This filter needs the POSIX schema, which the server you are connected to does not carry." : "",
  })),
  SaveFilter: async () => "",
  ResetFilter: async () => "",
  DeleteFilter: async () => "",
  RestoreDefaultFilters: async () => "",
  ValidateFilter: async (_id: string, filter: string) => ({
    valid: true, expanded: filter.replace("{{now-90d:filetime}}", "133758734610000000"), error: "",
  }),
  StartSearch: async () => {
    // Deliver results the way Go does — in batches, over time — so the
    // interface is exercised the same way.
    setTimeout(() => {
      for (let batch = 0; batch < 4; batch++) {
        setTimeout(() => {
          const rows = Array.from({ length: 30 }, (_, i) => {
            const n = batch * 30 + i + 1;
            const name = `${people[n % people.length]} ${n}`;
            return {
              dn: `cn=${name},ou=people,ou=corp,${ROOT}`,
              icon: "user",
              cells: [name, titles[n % titles.length], `cn=${name},ou=people,ou=corp,${ROOT}`],
            };
          });
          emit("search:batch", { rows, matched: (batch + 1) * 30 });
          if (batch === 3) {
            emit("search:done", {
              matched: 120, truncated: false, cancelled: false,
              columns: ["cn", "title", "distinguishedName"], elapsed: 143,
            });
          }
        }, batch * 120);
      }
    }, 60);
    return { started: true, columns: ["cn", "title", "distinguishedName"], expanded: "" };
  },
  StopSearch: async () => {},
  ChooseExportPath: async () => "",
  ExportSearch: async () => "",
  ExportEntry: async () => "",
};

const listeners: Record<string, ((data: unknown) => void)[]> = {};

function emit(name: string, data: unknown) {
  for (const fn of listeners[name] ?? []) fn(data);
}

export const mockRuntime = {
  EventsOn: (name: string, fn: (data: unknown) => void) => {
    (listeners[name] ??= []).push(fn);
    return () => {};
  },
  // ?platform=windows shows the chrome Windows gets, which is the only way to
  // look at it without a Windows machine in front of you.
  Environment: async () => ({
    buildType: "dev",
    platform: new URLSearchParams(location.search).get("platform") ?? "darwin",
    arch: "arm64",
  }),
  WindowMinimise: () => {},
  WindowToggleMaximise: () => {},
  WindowIsMaximised: async () => false,
  Quit: () => {},
};

/** installed reports whether the mock is standing in for the real bindings. */
export const usingMock = typeof window !== "undefined" && !(window as { go?: unknown }).go;
