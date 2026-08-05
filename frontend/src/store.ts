import { create } from "zustand";
import * as api from "../wailsjs/go/app/App";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { app } from "../wailsjs/go/models";
import {
  EVENT_SEARCH_BATCH,
  EVENT_SEARCH_DONE,
  type SearchBatch,
  type SearchDone,
  type SearchRow,
} from "./events";

export type Mode = "browse" | "search" | "library";

/** A node plus what the tree knows about it: how deep it sits, whether it is
 *  open, and how much of it has been loaded. */
export interface TreeNode {
  node: app.Node;
  depth: number;
  expanded: boolean;
  /** loading is set while a page is in flight, so the row can say so. */
  loading: boolean;
  /** cookie is the cursor for the next page; empty once fully loaded. */
  cookie: string;
  /** page counts the pages fetched so far. */
  page: number;
  children: string[] | null;
}

interface State {
  connection: app.ConnectionState | null;
  profiles: app.ProfileSummary[];
  dialog: "connect" | "certificate" | null;
  certificate: app.CertPrompt | null;
  pendingProfileID: string | null;
  error: string;

  /** nodes is keyed by DN. A flat map is what keeps expanding one node from
   *  re-rendering its siblings. */
  nodes: Record<string, TreeNode>;
  roots: string[];
  selected: string | null;
  detail: app.EntryDetail | null;
  detailError: string;

  loadProfiles: () => Promise<void>;
  openConnect: () => void;
  closeDialog: () => void;
  connect: (profileID: string, password: string) => Promise<void>;
  disconnect: () => Promise<void>;
  trustAndRetry: () => Promise<void>;
  toggle: (dn: string) => Promise<void>;
  loadMore: (dn: string) => Promise<void>;
  select: (dn: string) => Promise<void>;

  mode: Mode;
  filterText: string;
  scope: string;
  searching: boolean;
  results: SearchRow[];
  columns: string[];
  matched: number;
  searchNote: string;

  library: app.FilterSummary[];
  editing: app.FilterSummary | null;
  check: app.FilterCheck | null;

  setMode: (m: Mode) => void;
  setFilterText: (s: string) => void;
  setScope: (s: string) => void;
  runSearch: () => Promise<void>;
  stopSearch: () => Promise<void>;
  exportResults: () => Promise<void>;

  loadLibrary: () => Promise<void>;
  editFilter: (f: app.FilterSummary | null) => void;
  checkFilter: (s: string) => Promise<void>;
  saveFilter: (f: app.FilterInput) => Promise<void>;
  resetFilter: (id: string) => Promise<void>;
  deleteFilter: (id: string) => Promise<void>;
  restoreFilters: () => Promise<void>;
  useFilter: (f: app.FilterSummary) => void;
}

/** PAGE_SIZE is deliberately smaller than Active Directory's own 1000: the
 *  first rows appear sooner, and the tree stays responsive while the rest
 *  arrives. */
const PAGE_SIZE = 200;

function emptyTree() {
  return { nodes: {}, roots: [], selected: null, detail: null, detailError: "" };
}

export const useStore = create<State>((set, get) => ({
  connection: null,
  profiles: [],
  dialog: null,
  certificate: null,
  pendingProfileID: null,
  error: "",
  ...emptyTree(),

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

  loadProfiles: async () => {
    set({ profiles: await api.ListProfiles() });
  },

  openConnect: () => set({ dialog: "connect", error: "" }),
  closeDialog: () => set({ dialog: null, certificate: null }),

  connect: async (profileID, password) => {
    const result = await api.Connect(profileID, password);

    if (result.certificate) {
      set({
        dialog: "certificate",
        certificate: result.certificate,
        pendingProfileID: profileID,
      });
      return;
    }
    if (result.error) {
      set({ error: result.error, dialog: "connect" });
      return;
    }

    set({
      connection: result.state,
      dialog: null,
      certificate: null,
      error: "",
      ...emptyTree(),
    });
    await get().loadProfiles();

    // Open the root branch straight away, so the window is never an empty
    // pane the user has to guess their way out of.
    const rootDN = result.state.rootDN;
    if (!rootDN) return;

    set((s) => ({
      nodes: {
        ...s.nodes,
        [rootDN]: {
          node: {
            dn: rootDN,
            label: rootDN,
            rdn: rootDN,
            icon: "domain",
            childCount: -1,
            hasChildren: true,
          } as app.Node,
          depth: 0,
          expanded: false,
          loading: false,
          cookie: "",
          page: 0,
          children: null,
        },
      },
      roots: [rootDN],
    }));
    await get().toggle(rootDN);
  },

  disconnect: async () => {
    const id = get().connection?.profileId;
    if (id) await api.Disconnect(id);
    set({ connection: null, ...emptyTree() });
    await get().loadProfiles();
  },

  trustAndRetry: async () => {
    const { certificate, pendingProfileID } = get();
    if (!certificate || !pendingProfileID) return;

    const err = await api.TrustCertificate(pendingProfileID, certificate.fingerprint);
    if (err) {
      set({ error: err, dialog: "connect", certificate: null });
      return;
    }
    set({ certificate: null, dialog: null });
    await get().connect(pendingProfileID, "");
  },

  toggle: async (dn) => {
    const entry = get().nodes[dn];
    if (!entry) return;

    if (entry.expanded) {
      set((s) => ({ nodes: { ...s.nodes, [dn]: { ...s.nodes[dn], expanded: false } } }));
      return;
    }
    if (entry.children !== null) {
      set((s) => ({ nodes: { ...s.nodes, [dn]: { ...s.nodes[dn], expanded: true } } }));
      return;
    }
    await get().loadMore(dn);
  },

  loadMore: async (dn) => {
    const { connection, nodes } = get();
    const entry = nodes[dn];
    if (!connection || !entry || entry.loading) return;

    set((s) => ({
      nodes: { ...s.nodes, [dn]: { ...s.nodes[dn], loading: true, expanded: true } },
    }));

    const result = await api.Children(connection.profileId, dn, PAGE_SIZE, entry.cookie);

    if (result.error) {
      set((s) => ({
        error: result.error,
        nodes: { ...s.nodes, [dn]: { ...s.nodes[dn], loading: false } },
      }));
      return;
    }

    set((s) => {
      const parent = s.nodes[dn];
      if (!parent) return {};

      const added: Record<string, TreeNode> = {};
      for (const node of result.nodes ?? []) {
        added[node.dn] = {
          node,
          depth: parent.depth + 1,
          expanded: false,
          loading: false,
          cookie: "",
          page: 0,
          children: null,
        };
      }

      return {
        nodes: {
          ...s.nodes,
          ...added,
          [dn]: {
            ...parent,
            loading: false,
            expanded: true,
            cookie: result.cookie,
            page: parent.page + 1,
            children: [...(parent.children ?? []), ...(result.nodes ?? []).map((n) => n.dn)],
          },
        },
      };
    });
  },

  select: async (dn) => {
    const { connection } = get();
    set({ selected: dn, detail: null, detailError: "" });
    if (!connection) return;

    const result = await api.Entry(connection.profileId, dn);
    // Ignore a result that arrived after the user clicked somewhere else.
    if (get().selected !== dn) return;

    if (result.error) {
      set({ detailError: result.error });
      return;
    }
    set({ detail: result.detail });
  },

  setMode: (mode) => {
    set({ mode });
    if (mode === "library") void get().loadLibrary();
  },
  setFilterText: (filterText) => set({ filterText }),
  setScope: (scope) => set({ scope }),

  runSearch: async () => {
    const { connection, filterText, scope } = get();
    if (!connection) return;

    // Clear before starting: results from the previous search appended to the
    // new ones would be indistinguishable.
    set({ results: [], matched: 0, searching: true, searchNote: "searching…", error: "" });

    const started = await api.StartSearch({
      profileId: connection.profileId,
      filter: filterText,
      scope,
      base: "",
      columns: [],
    } as app.SearchInput);

    if (!started.started) {
      set({ searching: false, searchNote: "", error: started.error });
      return;
    }
    set({ columns: started.columns ?? [] });
  },

  stopSearch: async () => {
    const id = get().connection?.profileId;
    if (id) await api.StopSearch(id);
  },

  exportResults: async () => {
    const { connection, filterText, scope } = get();
    if (!connection) return;

    const path = await api.ChooseExportPath("results.ldif");
    if (!path) return; // cancelled

    const msg = await api.ExportSearch({
      profileId: connection.profileId,
      path,
      filter: filterText,
      scope,
      base: "",
      columns: [],
    } as app.ExportInput);

    set(msg ? { error: msg } : { searchNote: `exported to ${path}` });
  },

  loadLibrary: async () => {
    const id = get().connection?.profileId ?? "";
    set({ library: await api.ListFilters(id) });
  },

  editFilter: (editing) => set({ editing, check: null }),

  checkFilter: async (filter) => {
    const id = get().connection?.profileId ?? "";
    set({ check: await api.ValidateFilter(id, filter) });
  },

  saveFilter: async (f) => {
    const msg = await api.SaveFilter(f);
    if (msg) {
      set({ error: msg });
      return;
    }
    set({ error: "", editing: null });
    await get().loadLibrary();
  },

  resetFilter: async (id) => {
    const msg = await api.ResetFilter(id);
    if (msg) {
      set({ error: msg });
      return;
    }
    set({ editing: null });
    await get().loadLibrary();
  },

  deleteFilter: async (id) => {
    const msg = await api.DeleteFilter(id);
    if (msg) {
      set({ error: msg });
      return;
    }
    set({ editing: null });
    await get().loadLibrary();
  },

  restoreFilters: async () => {
    const msg = await api.RestoreDefaultFilters();
    if (msg) {
      set({ error: msg });
      return;
    }
    set({ editing: null });
    await get().loadLibrary();
  },

  useFilter: (f) => {
    set({ mode: "search", filterText: f.filter, scope: f.scope || "subtree" });
    void get().runSearch();
  },
}));

// Results arrive as events. Subscribing once at module load is what keeps a
// forty-thousand-row search from being one frozen call.
EventsOn(EVENT_SEARCH_BATCH, (batch: SearchBatch) => {
  useStore.setState((s) => ({
    results: [...s.results, ...(batch.rows ?? [])],
    matched: batch.matched,
  }));
});

EventsOn(EVENT_SEARCH_DONE, (done: SearchDone) => {
  useStore.setState({
    searching: false,
    matched: done.matched,
    columns: done.columns ?? [],
    searchNote: done.error
      ? done.error
      : done.cancelled
        ? `stopped after ${done.matched}`
        : done.truncated
          ? done.reason
          : `${done.matched} matched in ${done.elapsed} ms`,
  });
});

/** visibleRows flattens the tree into exactly the rows on screen, in order.
 *
 *  It is a plain function over the two pieces of state it needs rather than a
 *  Zustand selector. A selector that builds a new array on every call returns
 *  a new reference each time, and Zustand v5 treats that as an unstable
 *  snapshot and throws — which unmounts the whole window, not just the tree.
 *  Components select `nodes` and `roots` and memoise this instead. */
export function visibleRows(
  nodes: Record<string, TreeNode>,
  roots: string[],
): TreeNode[] {
  const out: TreeNode[] = [];

  const walk = (dn: string) => {
    const entry = nodes[dn];
    if (!entry) return;
    out.push(entry);
    if (!entry.expanded || !entry.children) return;
    for (const child of entry.children) walk(child);
  };

  for (const root of roots) walk(root);
  return out;
}
