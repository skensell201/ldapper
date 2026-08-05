import { useEffect } from "react";
import { Tree } from "./panes/Tree";
import { Detail } from "./panes/Detail";
import { Connect } from "./dialogs/Connect";
import { Certificate } from "./dialogs/Certificate";
import { useStore } from "./store";
import "./App.css";

export function App() {
  const state = useStore((s) => s.connection);
  const dialog = useStore((s) => s.dialog);
  const openConnect = useStore((s) => s.openConnect);
  const loadProfiles = useStore((s) => s.loadProfiles);

  useEffect(() => {
    void loadProfiles();
  }, [loadProfiles]);

  return (
    <div className="window">
      <header className="utility drag">
        <span className="wordmark">Ldapper</span>
        <button className="conn no-drag" onClick={openConnect}>
          <i className={state?.connected ? "led live" : "led"} />
          {state?.connected
            ? `${state.host} · ${state.boundAs || "anonymous"}`
            : "not connected"}
          <span className="chev">▾</span>
        </button>
        <span className="spacer" />
        <button className="util no-drag" onClick={openConnect}>
          Connections
        </button>
      </header>

      <main className="workspace">
        <Tree />
        <Detail />
      </main>

      <footer className="statusbar">
        {state?.connected ? (
          <>
            <span className="live">Connected</span>
            <span>{state.encryption === "none" ? "unencrypted" : state.encryption.toUpperCase()}</span>
            <span>{state.isActiveDirectory ? "Active Directory" : "LDAP"}</span>
            <span>{state.supportsPaging ? "paged results" : "no paging control"}</span>
            <span className="spacer" />
            <span>{state.rootDN}</span>
          </>
        ) : (
          <span>idle</span>
        )}
      </footer>

      {dialog === "connect" && <Connect />}
      {dialog === "certificate" && <Certificate />}
    </div>
  );
}
