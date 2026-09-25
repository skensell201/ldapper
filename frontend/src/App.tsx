import { useEffect, useState } from "react";
import { Tree } from "./panes/Tree";
import { Detail } from "./panes/Detail";
import { Results } from "./panes/Results";
import { Library } from "./panes/Library";
import { Toolbar } from "./Toolbar";
import { Connect } from "./dialogs/Connect";
import { Certificate } from "./dialogs/Certificate";
import { useStore, type Mode } from "./store";
import { shortIdentity } from "./identity";
import { healthLabel, healthOf } from "./health";
import { Mark } from "./Mark";
import { WindowControls } from "./WindowControls";
import { api, Environment, WindowToggleMaximise } from "./api";

import type { app } from "../wailsjs/go/models";
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

  // macOS puts its window controls inside our own chrome, so the title bar
  // needs room for them on the left. Windows and Linux do not, and reserving
  // it there just pushed the name off centre.
  const [platform, setPlatform] = useState("");
  const [build, setBuild] = useState<app.BuildInfo | null>(null);
  useEffect(() => {
    void Environment().then((e) => setPlatform(e.platform));
    void api.Build().then(setBuild);
  }, []);

  useEffect(() => {
    void loadProfiles();
  }, [loadProfiles]);

  const health = healthOf(state, error);

  return (
    <div className={mode === "browse" ? "window" : "window with-toolbar"}>
      <header
        className={`utility drag${platform === "darwin" ? " mac" : ""}${platform === "windows" ? " win" : ""}`}
        onDoubleClick={() => {
          // Double-clicking a title bar maximises the window everywhere. Ours
          // has to do it itself on Windows, where there is no title bar left.
          if (platform === "windows") void WindowToggleMaximise();
        }}
      >
        <span className="brand">
          <Mark size={20} />
          <span className="wordmark">Ldapper</span>
        </span>

        <button
          className="conn no-drag"
          onClick={openConnect}
          title={
            state?.connected
              ? `${healthLabel(health, state, error)} — ${state.host}, ${state.boundAs || "anonymous"}`
              : healthLabel(health, state, error)
          }
        >
          <i className={`led ${health}`} />
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
        {platform === "windows" && <WindowControls />}
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
        <span className={`state ${health}`} title={healthLabel(health, state, error)}>
          <i className={`led ${health}`} />
          {healthLabel(health, state, error)}
        </span>

        {state?.connected && !error && (
          <>
            <span className={state.encryption === "none" ? "warn-text" : ""}>
              {state.encryption === "none" ? "no TLS" : state.encryption.toUpperCase()}
            </span>
            <span>{state.isActiveDirectory ? "Active Directory" : "LDAP"}</span>
            <span>{state.supportsPaging ? "paged results" : "no paging control"}</span>
            <span title={state.rootDN}>{state.rootDN}</span>
          </>
        )}

        <span className="spacer" />
        {build && (
          <span
            className="build"
            title={
              build.commit
                ? `Ldapper ${build.version}, built from ${build.commit}${build.modified ? " with local changes" : ""}`
                : `Ldapper ${build.version}`
            }
          >
            {build.version}
            {build.modified ? "+" : ""}
          </span>
        )}
      </footer>

      {dialog === "connect" && <Connect />}
      {dialog === "certificate" && <Certificate />}
    </div>
  );
}
