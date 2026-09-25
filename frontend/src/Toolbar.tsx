import { useStore } from "./store";
import "./Toolbar.css";

const SCOPES = [
  { value: "base", label: "Base" },
  { value: "one", label: "One level" },
  { value: "subtree", label: "Subtree" },
];

export function Toolbar() {
  const mode = useStore((s) => s.mode);
  const connected = useStore((s) => s.connection?.connected ?? false);
  const filterText = useStore((s) => s.filterText);
  const scope = useStore((s) => s.scope);
  const searching = useStore((s) => s.searching);
  const setFilterText = useStore((s) => s.setFilterText);
  const setScope = useStore((s) => s.setScope);
  const runSearch = useStore((s) => s.runSearch);
  const stopSearch = useStore((s) => s.stopSearch);
  const exportResults = useStore((s) => s.exportResults);
  const restoreFilters = useStore((s) => s.restoreFilters);
  const editFilter = useStore((s) => s.editFilter);

  if (mode === "library") {
    return (
      <div className="toolbar">
        <span className="toolbar-note">
          Every filter is editable. Reset returns a built-in one to the version Ldapper ships.
        </span>
        <span className="spacer" />
        <button
          className="tbtn"
          onClick={() =>
            editFilter({
              id: "",
              name: "New filter",
              description: "",
              filter: "(objectClass=*)",
              scope: "subtree",
              base: "",
              columns: [],
              dialect: "generic",
              builtIn: false,
              modified: false,
              supported: true,
              reason: "",
            } as never)
          }
        >
          New filter
        </button>
        <button className="tbtn plain" onClick={() => void restoreFilters()}>
          Restore all defaults
        </button>
      </div>
    );
  }

  return (
    <div className="toolbar">
      <input
        className="filter-box"
        value={filterText}
        onChange={(e) => setFilterText(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && connected && !searching) void runSearch();
        }}
        placeholder="(&(objectClass=user)(sAMAccountName=a.*))"
        aria-label="LDAP filter"
        spellCheck={false}
      />

      <div className="segs small">
        {SCOPES.map((o) => (
          <button
            key={o.value}
            className={scope === o.value ? "seg on" : "seg"}
            onClick={() => setScope(o.value)}
          >
            {o.label}
          </button>
        ))}
      </div>

      {searching ? (
        <button className="tbtn" onClick={() => void stopSearch()}>
          Stop
        </button>
      ) : (
        <button className="tbtn primary" onClick={() => void runSearch()} disabled={!connected}>
          Search
        </button>
      )}
      <button className="tbtn plain" onClick={() => void exportResults()} disabled={!connected}>
        Export
      </button>
    </div>
  );
}
