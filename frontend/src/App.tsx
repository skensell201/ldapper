import { useEffect } from "react";
import { Tree } from "./panes/Tree";
import { Detail } from "./panes/Detail";
import { Results } from "./panes/Results";
import { Library } from "./panes/Library";
import { Toolbar } from "./Toolbar";
import { Connect } from "./dialogs/Connect";
import { Certificate } from "./dialogs/Certificate";
import { useStore, type Mode } from "./store";
import { shortIdentity } from "./identity";
import "./App.css";

const MODES: { value: Mode; label: string }[] = [
  { value: "browse", label: "Browse" },
  { value: "search", label: "Search" },
  { value: "library", label: "Filters" },
];

export function App() {
  const state = useStore((s) => s.connection);
  const dialog = useStore((s) => s.dialog);
  const mode = useStore((s) => s.mode);
  const error = useStore((s) => s.error);
  const openConnect = useStore((s) => s.openConnect);
  const setMode = useStore((s) => s.setMode);
  const loadProfiles = useStore((s) => s.loadProfiles);

  useEffect(() => {
    void loadProfiles();
  }, [loadProfiles]);

  return (
    <div className={mode === "browse" ? "window" : "window with-toolbar"}>
      <header className="utility drag">
        <span className="wordmark">Ldapper</span>

        <button
          className="conn no-drag"
          onClick={openConnect}
          title={state?.connected ? `${state.host} — ${state.boundAs || "anonymous"}` : "Not connected"}
        >
          <i className={state?.connected ? "led live" : "led"} />
          {state?.connected ? `${state.host} · ${shortIdentity(state.boundAs)}` : "not connected"}
          <span className="chev">▾</span>
        </button>

        <div className="segs small no-drag modes">
          {MODES.map((m) => (
            <button
              key={m.value}
              className={mode === m.value ? "seg on" : "seg"}
              onClick={() => setMode(m.value)}
            >
              {m.label}
            </button>
          ))}
        </div>

        <span className="spacer" />
        <button className="util no-drag" onClick={openConnect}>
          Connections
        </button>
      </header>

      {mode !== "browse" && <Toolbar />}

      <main className="workspace">
        {mode === "browse" && (
          <>
            <Tree />
            <Detail />
          </>
        )}
        {mode === "search" && (
          <>
            <Tree />
            <Results />
          </>
        )}
        {mode === "library" && <Library />}
      </main>

      <footer className="statusbar">
        {error ? (
          <span className="state bad">
            <i className="led warn" />
            {error}
          </span>
        ) : state?.connected ? (
          <>
            <span className="state good">
              <i className="led live" />
              Connected
            </span>
            <span className={state.encryption === "none" ? "warnish" : ""}>
              {state.encryption === "none" ? "unencrypted" : state.encryption.toUpperCase()}
            </span>
            <span>{state.isActiveDirectory ? "Active Directory" : "LDAP"}</span>
            <span>{state.supportsPaging ? "paged results" : "no paging control"}</span>
            <span className="spacer" />
            <span title={state.rootDN}>{state.rootDN}</span>
          </>
        ) : (
          <span className="state">
            <i className="led" />
            not connected
          </span>
        )}
      </footer>

      {dialog === "connect" && <Connect />}
      {dialog === "certificate" && <Certificate />}
    </div>
  );
}
