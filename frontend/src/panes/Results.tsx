import { useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useStore } from "../store";
import { Icon } from "../Icon";
import "./Results.css";

export function Results() {
  const rows = useStore((s) => s.results);
  const columns = useStore((s) => s.columns);
  const matched = useStore((s) => s.matched);
  const searching = useStore((s) => s.searching);
  const note = useStore((s) => s.searchNote);
  const select = useStore((s) => s.select);
  const setMode = useStore((s) => s.setMode);

  const scrollRef = useRef<HTMLDivElement>(null);
  // The same reason as the tree: a subtree search can match tens of thousands
  // of objects, and rendering that many rows is not survivable.
  const virtual = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 30,
    overscan: 14,
  });

  return (
    <section className="pane">
      <div className="stream">
        <span>
          {searching ? "searching — " : ""}
          <span className="n">{matched}</span> matched
        </span>
        {searching && (
          <span className="bar">
            <i />
          </span>
        )}
        <span className="note">{note}</span>
      </div>

      {rows.length === 0 ? (
        <div className="empty">{searching ? "waiting for the first results…" : "no results"}</div>
      ) : (
        <>
          <div className="res-head" style={{ gridTemplateColumns: gridFor(columns) }}>
            {columns.map((c) => (
              <span key={c}>{c}</span>
            ))}
          </div>

          <div className="res-scroll" ref={scrollRef}>
            <div className="res-inner" style={{ height: virtual.getTotalSize() }}>
              {virtual.getVirtualItems().map((item) => {
                const row = rows[item.index];
                return (
                  <div
                    className="res-row"
                    key={`${row.dn}-${item.index}`}
                    style={{
                      transform: `translateY(${item.start}px)`,
                      gridTemplateColumns: gridFor(columns),
                    }}
                    onClick={() => {
                      // Clicking a result opens it in the browse pane, which
                      // is where the full attribute list already lives.
                      setMode("browse");
                      void select(row.dn);
                    }}
                    title={row.dn}
                  >
                    {row.cells.map((cell: string, i: number) => (
                      <span className={i === 0 ? "cell first" : "cell"} key={i}>
                        {i === 0 && <Icon name={row.icon} className="res-icon" />}
                        {cell}
                      </span>
                    ))}
                  </div>
                );
              })}
            </div>
          </div>
        </>
      )}
    </section>
  );
}

/** The distinguished name is the widest column by far, so it takes the slack
 *  and everything before it gets a fixed share. */
function gridFor(columns: string[]): string {
  if (columns.length === 0) return "1fr";
  return `${"minmax(140px, 0.6fr) ".repeat(columns.length - 1)}minmax(220px, 1.6fr)`;
}
