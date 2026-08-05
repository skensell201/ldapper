import { describe, expect, it, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { api, handlers } from "../test/wails";
import { useStore } from "../store";
import { Tree } from "./Tree";
import { Detail } from "./Detail";
import { Results } from "./Results";
import { Library } from "./Library";
import { App } from "../App";

/** The state a freshly started window is in. Every test starts from here so
 *  one cannot leak into the next. */
function reset() {
  useStore.setState({
    connection: null,
    profiles: [],
    dialog: null,
    certificate: null,
    error: "",
    nodes: {},
    roots: [],
    selected: null,
    detail: null,
    detailError: "",
    mode: "browse",
    filterText: "(objectClass=*)",
    scope: "subtree",
    searching: false,
    results: [],
    columns: [],
    matched: 0,
    searchNote: "",
    library: [],
    editing: null,
    check: null,
  });
}

const connected = {
  profileId: "dev",
  connected: true,
  host: "localhost",
  boundAs: "cn=admin,dc=example,dc=com",
  rootDN: "dc=example,dc=com",
  isActiveDirectory: false,
  supportsPaging: true,
  dialects: ["generic", "posix"],
  encryption: "none",
};

function node(dn: string, label: string, extra = {}) {
  return { dn, label, rdn: label, icon: "user", childCount: -1, hasChildren: false, ...extra };
}

function treeEntry(n: ReturnType<typeof node>, extra = {}) {
  return { node: n, depth: 0, expanded: false, loading: false, cookie: "", page: 0, children: null, ...extra };
}

beforeEach(() => {
  reset();
  Object.values(api).forEach((fn) => fn.mockClear?.());
});

describe("the tree", () => {
  it("says so when nothing is connected", () => {
    render(<Tree />);
    expect(screen.getByText("not connected")).toBeInTheDocument();
  });

  it("draws a row per node", () => {
    const a = node("cn=Anna,dc=example,dc=com", "Anna");
    const b = node("cn=Boris,dc=example,dc=com", "Boris");
    useStore.setState({
      connection: connected as never,
      nodes: { [a.dn]: treeEntry(a), [b.dn]: treeEntry(b) },
      roots: [a.dn, b.dn],
    });

    render(<Tree />);
    expect(screen.getByText("Anna")).toBeInTheDocument();
    expect(screen.getByText("Boris")).toBeInTheDocument();
  });

  // The blank-window bug was a render crash in this component. A test that
  // renders it with real data is what would have caught it.
  it("survives a node with every awkward field", () => {
    const odd = node("cn=Volkova\\2C Anna,dc=example,dc=com", "Volkova, Anna", {
      icon: "not-an-icon-we-draw",
      childCount: 0,
      hasChildren: false,
    });
    useStore.setState({
      connection: connected as never,
      nodes: { [odd.dn]: treeEntry(odd) },
      roots: [odd.dn],
    });

    render(<Tree />);
    expect(screen.getByText("Volkova, Anna")).toBeInTheDocument();
  });

  // A cookie means the server has more to give. Saying which page comes next
  // beats a spinner that never ends.
  it("offers the next page when the server has more", async () => {
    const n = node("ou=people,dc=example,dc=com", "people", { hasChildren: true, icon: "ou" });
    useStore.setState({
      connection: connected as never,
      nodes: { [n.dn]: treeEntry(n, { expanded: true, cookie: "Y29va2ll", page: 1, children: [] }) },
      roots: [n.dn],
    });

    api.Children.mockResolvedValue({ nodes: [], cookie: "", error: "" });

    render(<Tree />);
    const more = screen.getByRole("button", { name: /page 2/ });
    expect(more).toBeInTheDocument();

    await userEvent.click(more);
    expect(api.Children).toHaveBeenCalledWith("dev", n.dn, expect.any(Number), "Y29va2ll");
  });

  it("says a page is loading instead of going quiet", () => {
    const n = node("ou=people,dc=example,dc=com", "people", { hasChildren: true, icon: "ou" });
    useStore.setState({
      connection: connected as never,
      nodes: { [n.dn]: treeEntry(n, { loading: true, expanded: true }) },
      roots: [n.dn],
    });

    render(<Tree />);
    expect(screen.getByText("loading…")).toBeInTheDocument();
  });

  it("selects an object when its row is clicked", async () => {
    const n = node("cn=Anna,dc=example,dc=com", "Anna");
    api.Entry.mockResolvedValue({ detail: { dn: n.dn, rdn: "cn=Anna", icon: "user", rows: [] }, error: "" });
    useStore.setState({
      connection: connected as never,
      nodes: { [n.dn]: treeEntry(n) },
      roots: [n.dn],
    });

    render(<Tree />);
    await userEvent.click(screen.getByText("Anna"));
    expect(api.Entry).toHaveBeenCalledWith("dev", n.dn);
  });
});

describe("the attribute table", () => {
  it("asks for a selection before there is one", () => {
    render(<Detail />);
    expect(screen.getByText("select an object")).toBeInTheDocument();
  });

  it("shows the raw value and what it means", () => {
    useStore.setState({
      selected: "cn=Anna,dc=example,dc=com",
      detail: {
        dn: "cn=Anna,dc=example,dc=com",
        rdn: "cn=Anna",
        icon: "user",
        rows: [
          {
            name: "userAccountControl",
            values: [{ raw: "66048", decoded: ["NORMAL_ACCOUNT", "DONT_EXPIRE_PASSWORD"] }],
          },
          { name: "mail", values: [{ raw: "anna@example.com", decoded: [] }] },
        ],
      } as never,
    });

    render(<Detail />);
    // Both forms are on screen: the server's own value and its meaning.
    expect(screen.getByText("66048")).toBeInTheDocument();
    expect(screen.getByText("NORMAL_ACCOUNT")).toBeInTheDocument();
    expect(screen.getByText("DONT_EXPIRE_PASSWORD")).toBeInTheDocument();
    expect(screen.getByText("anna@example.com")).toBeInTheDocument();
  });

  it("gives every value of a multi-valued attribute its own row", () => {
    useStore.setState({
      selected: "cn=x,dc=example,dc=com",
      detail: {
        dn: "cn=x,dc=example,dc=com",
        rdn: "cn=x",
        icon: "user",
        rows: [
          {
            name: "objectClass",
            values: [{ raw: "top" }, { raw: "person" }, { raw: "inetOrgPerson" }],
          },
        ],
      } as never,
    });

    render(<Detail />);
    for (const v of ["top", "person", "inetOrgPerson"]) {
      expect(screen.getByText(v)).toBeInTheDocument();
    }
    // The attribute name is written once, against the first value.
    expect(screen.getAllByText("objectClass")).toHaveLength(1);
  });

  it("shows the reason when an object cannot be read", () => {
    useStore.setState({ selected: "cn=x,dc=example,dc=com", detailError: "This account does not have permission." });
    render(<Detail />);
    expect(screen.getByText("This account does not have permission.")).toBeInTheDocument();
  });
});

describe("search results", () => {
  it("says nothing was found rather than showing an empty table", () => {
    render(<Results />);
    expect(screen.getByText("no results")).toBeInTheDocument();
  });

  it("draws a column per requested attribute and a row per result", () => {
    useStore.setState({
      columns: ["cn", "mail", "distinguishedName"],
      matched: 2,
      searchNote: "2 matched in 41 ms",
      results: [
        { dn: "cn=Anna,dc=example,dc=com", icon: "user", cells: ["Anna", "anna@example.com", "cn=Anna,dc=example,dc=com"] },
        { dn: "cn=Boris,dc=example,dc=com", icon: "user", cells: ["Boris", "", "cn=Boris,dc=example,dc=com"] },
      ],
    });

    render(<Results />);
    expect(screen.getByText("cn")).toBeInTheDocument();
    expect(screen.getByText("Anna")).toBeInTheDocument();
    expect(screen.getByText("Boris")).toBeInTheDocument();
    expect(screen.getByText("2 matched in 41 ms")).toBeInTheDocument();
  });

  // Results arrive as events, which is the part no return value can express.
  it("appends a batch that arrives from Go", () => {
    useStore.setState({ searching: true, columns: ["cn", "distinguishedName"] });

    handlers["search:batch"]({
      rows: [{ dn: "cn=Anna,dc=example,dc=com", icon: "user", cells: ["Anna", "cn=Anna,dc=example,dc=com"] }],
      matched: 1,
    });

    render(<Results />);
    expect(screen.getByText("Anna")).toBeInTheDocument();
    expect(useStore.getState().matched).toBe(1);
  });

  it("reports a truncated search as a fact, keeping what arrived", () => {
    useStore.setState({ searching: true });
    handlers["search:batch"]({
      rows: [{ dn: "cn=Anna,dc=example,dc=com", icon: "user", cells: ["Anna"] }],
      matched: 1,
    });
    handlers["search:done"]({
      matched: 1,
      truncated: true,
      reason: "The server reached its size limit — these are the first results, not all of them.",
      cancelled: false,
      columns: ["cn"],
      elapsed: 12,
    });

    render(<Results />);
    expect(useStore.getState().searching).toBe(false);
    expect(screen.getByText(/size limit/)).toBeInTheDocument();
    expect(screen.getByText("Anna")).toBeInTheDocument();
  });

  it("says a search was stopped rather than that it found nothing", () => {
    handlers["search:done"]({
      matched: 7, truncated: false, cancelled: true, columns: ["cn"], elapsed: 300,
    });
    expect(useStore.getState().searchNote).toMatch(/stopped after 7/);
  });
});

describe("the filter library", () => {
  const filters = [
    {
      id: "ad-disabled-accounts", name: "Disabled accounts", description: "Switched-off accounts.",
      filter: "(objectClass=user)", scope: "subtree", base: "", columns: ["cn"],
      dialect: "ad", builtIn: true, modified: false, supported: false,
      reason: "This filter uses an Active Directory extension.",
    },
    {
      id: "all-people", name: "All people", description: "Everyone.",
      filter: "(objectClass=person)", scope: "subtree", base: "", columns: ["cn"],
      dialect: "generic", builtIn: true, modified: true, supported: true, reason: "",
    },
    {
      id: "mine", name: "My filter", description: "Mine.",
      filter: "(objectClass=*)", scope: "subtree", base: "", columns: ["cn"],
      dialect: "generic", builtIn: false, modified: false, supported: true, reason: "",
    },
  ];

  it("groups the built-in filters apart from the user's own", async () => {
    api.ListFilters.mockResolvedValue(filters as never);
    useStore.setState({ connection: connected as never, library: filters as never });

    render(<Library />);
    expect(await screen.findByText("Built-in · Active Directory")).toBeInTheDocument();
    expect(screen.getByText("Built-in · any directory")).toBeInTheDocument();
    expect(screen.getByText("Mine")).toBeInTheDocument();
  });

  // Greying a filter the server cannot answer tells the user something.
  // Hiding it tells them nothing.
  it("greys an unavailable filter instead of hiding it", () => {
    useStore.setState({ connection: connected as never, library: filters as never });
    render(<Library />);

    const row = screen.getByText("Disabled accounts").closest(".row");
    expect(row).not.toBeNull();
    expect(row).toHaveClass("off");
    expect(row).toHaveAttribute("title", expect.stringContaining("Active Directory extension"));
  });

  it("marks a built-in that has been edited", () => {
    useStore.setState({ connection: connected as never, library: filters as never });
    render(<Library />);

    const row = screen.getByText("All people").closest(".row")!;
    expect(within(row as HTMLElement).getByTitle("edited")).toBeInTheDocument();
  });

  it("opens the editor and validates what is typed", async () => {
    api.ValidateFilter.mockResolvedValue({ valid: true, expanded: "(objectClass=person)", error: "" });
    useStore.setState({ connection: connected as never, library: filters as never });

    render(<Library />);
    await userEvent.click(screen.getByText("All people"));

    expect(await screen.findByDisplayValue("(objectClass=person)")).toBeInTheDocument();
    expect(api.ValidateFilter).toHaveBeenCalled();
    // An edited built-in offers a way back to the shipped version.
    expect(screen.getByRole("button", { name: "Reset" })).toBeInTheDocument();
  });

  it("refuses to save a filter that will not compile", async () => {
    api.ValidateFilter.mockResolvedValue({ valid: false, error: "not a valid LDAP filter", expanded: "" });
    useStore.setState({ connection: connected as never, library: filters as never });

    render(<Library />);
    await userEvent.click(screen.getByText("My filter"));

    expect(await screen.findByText(/not a valid LDAP filter/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
  });
});

describe("the window", () => {
  it("starts on Browse and switches modes", async () => {
    api.ListProfiles.mockResolvedValue([] as never);
    api.ListFilters.mockResolvedValue([] as never);

    render(<App />);
    expect(screen.getByText("select an object")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Search" }));
    expect(screen.getByText("no results")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Filters" }));
    expect(await screen.findByText("select a filter")).toBeInTheDocument();
  });

  it("shows what a connection is before anything is connected", () => {
    render(<App />);
    // The phrase appears twice on purpose: once in the chrome, once where the
    // tree would be.
    expect(screen.getAllByText("not connected")).toHaveLength(2);
    expect(screen.getByText("idle")).toBeInTheDocument();
  });

  // A binding that returns nothing must not leave the window silently empty.
  it("says so when the directory does not answer", async () => {
    const n = node("ou=people,dc=example,dc=com", "people", { hasChildren: true, icon: "ou" });
    useStore.setState({
      connection: connected as never,
      nodes: { [n.dn]: treeEntry(n, { expanded: true, cookie: "Y29va2ll", page: 1, children: [] }) },
      roots: [n.dn],
    });
    api.Children.mockResolvedValue(undefined);

    render(<Tree />);
    await userEvent.click(screen.getByRole("button", { name: /page 2/ }));

    expect(useStore.getState().error).toMatch(/did not answer/);
  });

  it("describes the connection in the status bar", () => {
    useStore.setState({ connection: connected as never });
    render(<App />);
    expect(screen.getByText("Connected")).toBeInTheDocument();
    expect(screen.getByText("unencrypted")).toBeInTheDocument();
    expect(screen.getByText("LDAP")).toBeInTheDocument();
    expect(screen.getByText("paged results")).toBeInTheDocument();
    expect(screen.getByText("dc=example,dc=com")).toBeInTheDocument();
  });
});
