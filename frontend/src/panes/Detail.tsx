import { useMemo, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useStore } from "../store";
import { Icon } from "../Icon";
import "./Detail.css";

/** A row of the table is one value, not one attribute: an attribute with
 *  seven values needs seven rows for the virtualiser to measure. */
interface ValueRow {
  attribute: string;
  first: boolean;
  raw: string;
  decoded: string[];
}

/** valueOf strips the attribute name from an RDN so the heading reads
 *  "Anna Volkova" rather than "CN=Anna Volkova". */
function valueOf(rdn: string): string {
  const i = rdn.indexOf("=");
  return i >= 0 ? rdn.slice(i + 1) : rdn;
}

export function Detail() {
  const detail = useStore((s) => s.detail);
  const error = useStore((s) => s.detailError);
  const selected = useStore((s) => s.selected);

  const rows = useMemo<ValueRow[]>(() => {
    if (!detail) return [];
    const out: ValueRow[] = [];
    for (const row of detail.rows ?? []) {
      for (const [i, value] of (row.values ?? []).entries()) {
        out.push({
          attribute: row.name,
          first: i === 0,
          raw: value.raw,
          decoded: value.decoded ?? [],
        });
      }
    }
    return out;
  }, [detail]);

  const scrollRef = useRef<HTMLDivElement>(null);
  const virtual = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    // A decoded value adds a second line, so rows are not all one height.
    // measureElement corrects the estimate once a row is on screen.
    estimateSize: (i) => (rows[i]?.decoded.length ? 54 : 32),
    overscan: 10,
  });

  if (error) {
    return (
      <section className="pane">
        <div className="error">{error}</div>
      </section>
    );
  }
  if (!selected) {
    return (
      <section className="pane">
        <div className="empty">select an object</div>
      </section>
    );
  }
  if (!detail) {
    return (
      <section className="pane">
        <div className="empty">loading…</div>
      </section>
    );
  }

  return (
    <section className="pane">
      <header className="detail-head">
        <span className="avatar">
          <Icon name={detail.icon} />
        </span>
        <span className="detail-title">
          <span className="cn">{valueOf(detail.rdn)}</span>
          <span className="dn" title={detail.dn}>
            {detail.dn}
          </span>
        </span>
        <button className="ghost" onClick={() => void navigator.clipboard.writeText(detail.dn)}>
          Copy DN
        </button>
      </header>

      <div className="attr-head">
        <span>Attribute</span>
        <span>Value</span>
      </div>

      <div className="attr-scroll" ref={scrollRef}>
        <div className="attr-inner" style={{ height: virtual.getTotalSize() }}>
          {virtual.getVirtualItems().map((item) => {
            const row = rows[item.index];
            return (
              <div
                className="attr-row"
                key={`${row.attribute}-${item.index}`}
                ref={virtual.measureElement}
                data-index={item.index}
                style={{ transform: `translateY(${item.start}px)` }}
              >
                <span className="attr-name">{row.first ? row.attribute : ""}</span>
                <span className="attr-value">
                  <span className={row.decoded.length ? "raw" : "val"}>{row.raw}</span>
                  {row.decoded.length > 0 && (
                    <span className="decoded-list">
                      {row.decoded.map((d) => (
                        <span className="decoded" key={d}>
                          {d}
                        </span>
                      ))}
                    </span>
                  )}
                </span>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}
