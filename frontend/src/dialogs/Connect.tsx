import { useEffect, useState } from "react";
import { useStore } from "../store";
import { api } from "../api";
import "./Dialog.css";

const ENCRYPTIONS = [
  { value: "ldaps", label: "LDAPS" },
  { value: "starttls", label: "StartTLS" },
  { value: "none", label: "None" },
];

const BINDS = [
  { value: "ntlm", label: "NTLM" },
  { value: "simple", label: "Simple bind" },
  { value: "anonymous", label: "Anonymous" },
];

export function Connect() {
  const profiles = useStore((s) => s.profiles);
  const error = useStore((s) => s.error);
  const connection = useStore((s) => s.connection);
  const close = useStore((s) => s.closeDialog);
  const connect = useStore((s) => s.connect);
  const disconnect = useStore((s) => s.disconnect);
  const loadProfiles = useStore((s) => s.loadProfiles);

  // Which saved connection is being edited. Empty means a new one.
  const [selected, setSelected] = useState(profiles[0]?.id ?? "");
  const saved = profiles.find((p) => p.id === selected);

  const [name, setName] = useState(saved?.name ?? "My directory");
  const [host, setHost] = useState(saved?.host ?? "");
  const [port, setPort] = useState(String(saved?.port ?? 636));
  const [encryption, setEncryption] = useState(saved?.encryption || "ldaps");
  const [bindMethod, setBindMethod] = useState(saved?.bindMethod || "ntlm");
  const [username, setUsername] = useState(saved?.username ?? "");
  const [password, setPassword] = useState("");
  const [remember, setRemember] = useState(false);
  const [busy, setBusy] = useState(false);

  // Saved connections may arrive after this dialog mounts, and the fields are
  // initialised once. Without this, opening the dialog a moment too early
  // showed factory defaults over a connection that was already saved — with
  // nothing on screen to say so.
  useEffect(() => {
    if (selected || profiles.length === 0) return;
    load(profiles[0]);
  }, [profiles, selected]);

  function load(p: (typeof profiles)[number]) {
    setSelected(p.id);
    setName(p.name);
    setHost(p.host);
    setPort(String(p.port));
    setEncryption(p.encryption || "ldaps");
    setBindMethod(p.bindMethod || "ntlm");
    setUsername(p.username ?? "");
    setPassword("");
  }

  function blank() {
    setSelected("");
    setName("My directory");
    setHost("");
    setPort("636");
    setEncryption("ldaps");
    setBindMethod("ntlm");
    setUsername("");
    setPassword("");
  }

  // The profile ID is derived rather than asked for. Nobody wants to invent a
  // key, and host:port is already unique per server.
  const id = selected || `${host}:${port}`;

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
        className="modal wide no-drag"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
      >
        <h3>Connect to a directory</h3>
        <p className="sub">
          Saved on this machine. The password goes to the system keychain, never to the settings
          file.
        </p>

        {error && <div className="error">{error}</div>}

        {/* Two columns, so the whole form fits a window without scrolling:
            where to connect on the left, who to connect as on the right. */}
        <div className="connect-grid">
          <div className="form">
            {profiles.length > 0 && (
              <div>
                <span className="lbl">Saved connections</span>
                <div className="saved-list">
                  {profiles.map((p) => (
                    <button
                      key={p.id}
                      className={selected === p.id ? "saved on" : "saved"}
                      onClick={() => load(p)}
                    >
                      <i className={p.connected ? "led live" : "led"} />
                      {p.name || p.host}
                      <span className="where">
                        {p.host}:{p.port}
                      </span>
                    </button>
                  ))}
                  <button className={selected === "" ? "saved on" : "saved"} onClick={blank}>
                    + New
                  </button>
                </div>
              </div>
            )}

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
          </div>

          <div className="form">
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

            {bindMethod === "anonymous" ? (
              <p className="sub">
                Connecting without credentials. Most directories allow reading a part of the tree
                this way, and will simply show less.
              </p>
            ) : (
              <>
                <div>
                  <span className="lbl">User</span>
                  <input
                    className="field"
                    value={username}
                    onChange={(e) => setUsername(e.target.value)}
                    placeholder={
                      bindMethod === "ntlm" ? "CORP\\a.kensel" : "cn=admin,dc=example,dc=com"
                    }
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
              </>
            )}
          </div>
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
