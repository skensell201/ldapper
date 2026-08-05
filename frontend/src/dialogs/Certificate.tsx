import { useStore } from "../store";
import "./Dialog.css";

export function Certificate() {
  const cert = useStore((s) => s.certificate);
  const close = useStore((s) => s.closeDialog);
  const trust = useStore((s) => s.trustAndRetry);

  if (!cert) return null;

  return (
    <div className="scrim">
      <div className="modal no-drag" style={{ width: 500 }} role="dialog" aria-modal="true">
        <h3>This server's certificate isn't trusted</h3>
        <p className="sub">
          {cert.subject} signed its certificate with an authority this machine doesn't know.
          That is normal for an internal CA — and it also looks exactly like an interception.
        </p>

        <div className="cert">
          <div className="cert-head">Verify the fingerprint before you continue</div>
          <dl>
            <dt>SHA-256</dt>
            <dd className="fp">{cert.fingerprint}</dd>
            <dt>Subject</dt>
            <dd>{cert.subject}</dd>
            <dt>Issuer</dt>
            <dd>{cert.issuer}</dd>
            <dt>Valid until</dt>
            <dd>{cert.notAfter}</dd>
          </dl>
        </div>

        <div className="acts">
          <span className="spacer" />
          <button className="btn plain" onClick={close}>
            Cancel
          </button>
          {/* Deliberately not the accent colour: continuing past a warning
              should not look like the recommended action. */}
          <button className="btn light" onClick={() => void trust()}>
            Continue
          </button>
        </div>
      </div>
    </div>
  );
}
