import { useMemo, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { useStore, visibleRows } from "../store";
import { Icon } from "../Icon";
import "./Tree.css";

export function Tree() {
  // Select the two raw pieces of state and flatten them here. Selecting a
  // freshly built array straight from the store hands React a new snapshot
  // on every render, which Zustand v5 rejects outright.
  const nodes = useStore((s) => s.nodes);
  const roots = useStore((s) => s.roots);
  const rows = useMemo(() => visibleRows(nodes, roots), [nodes, roots]);
  const selected = useStore((s) => s.selected);
  const connected = useStore((s) => s.connection?.connected ?? false);
  const toggle = useStore((s) => s.toggle);
  const loadMore = useStore((s) => s.loadMore);
  const select = useStore((s) => s.select);

  const scrollRef = useRef<HTMLDivElement>(null);

  // Virtualising from the first commit is not premature: an organizational
  // unit holding fifty thousand accounts is ordinary, and rendering that many
  // rows is not survivable.
  const virtual = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 32,
    overscan: 12,
  });

  if (!connected) {
    return (
      <aside className="pane">
        <div className="pane-head">
          <span>Directory</span>
        </div>
        <div className="empty">not connected</div>
      </aside>
    );
  }

  return (
    <aside className="pane">
      <div className="pane-head">
        <span>Directory</span>
        <span className="n">{rows.length}</span>
      </div>

      <div className="tree-scroll" ref={scrollRef}>
        <div className="tree-inner" style={{ height: virtual.getTotalSize() }} role="tree">
          {virtual.getVirtualItems().map((item) => {
            const row = rows[item.index];
            const { node } = row;
            const isSelected = selected === node.dn;

            return (
              <div
                key={node.dn}
                className={isSelected ? "row selected" : "row"}
                style={{
                  transform: `translateY(${item.start}px)`,
                  paddingLeft: 10 + row.depth * 15,
                }}
                onClick={() => void select(node.dn)}
                onDoubleClick={() => void toggle(node.dn)}
                role="treeitem"
                aria-selected={isSelected}
                aria-expanded={node.hasChildren ? row.expanded : undefined}
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    void select(node.dn);
                  }
                  if (e.key === "ArrowRight" && !row.expanded) void toggle(node.dn);
                  if (e.key === "ArrowLeft" && row.expanded) void toggle(node.dn);
                }}
                title={node.dn}
              >
                <button
                  className="chev-btn"
                  onClick={(e) => {
                    e.stopPropagation();
                    void toggle(node.dn);
                  }}
                  tabIndex={-1}
                  aria-hidden={!node.hasChildren}
                >
                  {node.hasChildren ? (row.expanded ? "▾" : "▸") : ""}
                </button>

                <Icon name={node.icon} className="row-icon" />
                <span className="label">{node.label}</span>

                {row.loading && <span className="count">loading…</span>}

                {/* A cookie means the server has more to give. Saying which
                    page is next beats a spinner that never ends. */}
                {!row.loading && row.cookie && (
                  <button
                    className="more"
                    onClick={(e) => {
                      e.stopPropagation();
                      void loadMore(node.dn);
                    }}
                  >
                    page {row.page + 1}
                  </button>
                )}

                {!row.loading && !row.cookie && node.childCount >= 0 && (
                  <span className="count">{node.childCount}</span>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </aside>
  );
}
