import { useState } from "react";
import { useStore } from "../store";
import * as api from "../../wailsjs/go/app/App";
import "./Dialog.css";

const ENCRYPTIONS = [
  { value: "ldaps", label: "LDAPS" },
  { value: "starttls", label: "StartTLS" },
  { value: "none", label: "None" },
];

const BINDS = [
  { value: "ntlm", label: "NTLM" },
  { value: "simple", label: "Simple bind" },
];

export function Connect() {
  const profiles = useStore((s) => s.profiles);
  const error = useStore((s) => s.error);
  const connection = useStore((s) => s.connection);
  const close = useStore((s) => s.closeDialog);
  const connect = useStore((s) => s.connect);
  const disconnect = useStore((s) => s.disconnect);
  const loadProfiles = useStore((s) => s.loadProfiles);

  const saved = profiles[0];
  const [name, setName] = useState(saved?.name ?? "My directory");
  const [host, setHost] = useState(saved?.host ?? "");
  const [port, setPort] = useState(String(saved?.port ?? 636));
  const [encryption, setEncryption] = useState(saved?.encryption || "ldaps");
  const [bindMethod, setBindMethod] = useState(saved?.bindMethod || "ntlm");
  const [username, setUsername] = useState(saved?.username ?? "");
  const [password, setPassword] = useState("");
  const [remember, setRemember] = useState(false);
  const [busy, setBusy] = useState(false);

  // The profile ID is derived rather than asked for. Nobody wants to invent a
  // key, and host:port is already unique per server.
  const id = saved?.id ?? `${host}:${port}`;

  const submit = async () => {
    setBusy(true);
    try {
      const msg = await api.SaveProfile({
        id,
        name,
        host,
        port: Number(port) || 0,
        encryption,
        bindMethod,
        domain: "",
        username,
        password,
        rememberPassword: remember,
      });
      if (msg) {
        useStore.setState({ error: msg });
        return;
      }
      await loadProfiles();
      await connect(id, password);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="scrim" onClick={close}>
      <div
        className="modal no-drag"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
      >
        <h3>Connect to a directory</h3>
        <p className="sub">
          Saved on this machine. The password goes to the system keychain, never to the
          settings file.
        </p>

        {error && <div className="error">{error}</div>}

        <div className="form">
          <div>
            <span className="lbl">Name</span>
            <input
              className="field"
              value={name}
              onChange={(e) => setName(e.target.value)}
              aria-label="Connection name"
            />
          </div>

          <div className="cols">
            <div>
              <span className="lbl">Host</span>
              <input
                className="field"
                value={host}
                onChange={(e) => setHost(e.target.value)}
                placeholder="dc01.corp.example.com"
                aria-label="Host"
                autoFocus
              />
            </div>
            <div>
              <span className="lbl">Port</span>
              <input
                className="field"
                value={port}
                onChange={(e) => setPort(e.target.value)}
                inputMode="numeric"
                aria-label="Port"
              />
            </div>
          </div>

          <div>
            <span className="lbl">Encryption</span>
            <div className="segs">
              {ENCRYPTIONS.map((o) => (
                <button
                  key={o.value}
                  className={encryption === o.value ? "seg on" : "seg"}
                  onClick={() => setEncryption(o.value)}
                >
                  {o.label}
                </button>
              ))}
            </div>
          </div>

          <div>
            <span className="lbl">Authentication</span>
            <div className="segs">
              {BINDS.map((o) => (
                <button
                  key={o.value}
                  className={bindMethod === o.value ? "seg on" : "seg"}
                  onClick={() => setBindMethod(o.value)}
                >
                  {o.label}
                </button>
              ))}
            </div>
          </div>

          <div>
            <span className="lbl">User</span>
            <input
              className="field"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              placeholder={bindMethod === "ntlm" ? "CORP\\a.kensel" : "cn=admin,dc=example,dc=com"}
              aria-label="User"
            />
          </div>

          <div>
            <span className="lbl">Password</span>
            <input
              className="field"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              aria-label="Password"
            />
          </div>

          <label className="check">
            <input
              type="checkbox"
              checked={remember}
              onChange={(e) => setRemember(e.target.checked)}
            />
            Remember the password in this system&apos;s keychain
          </label>
        </div>

        <div className="acts">
          {connection?.connected && (
            <button className="btn plain" onClick={() => void disconnect()}>
              Disconnect
            </button>
          )}
          <span className="spacer" />
          <button className="btn plain" onClick={close}>
            Cancel
          </button>
          <button className="btn cta" onClick={() => void submit()} disabled={busy || !host}>
            {busy ? "Connecting…" : "Connect"}
          </button>
        </div>
      </div>
    </div>
  );
}
