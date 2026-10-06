import { useEffect, useState } from "react";
import { useStore } from "../store";
import { Icon } from "../Icon";
import type { app } from "../../wailsjs/go/models";
import "./Library.css";

export function Library() {
  const library = useStore((s) => s.library);
  const editing = useStore((s) => s.editing);
  const editFilter = useStore((s) => s.editFilter);
  const useFilter = useStore((s) => s.useFilter);
  const loadLibrary = useStore((s) => s.loadLibrary);

  useEffect(() => {
    void loadLibrary();
  }, [loadLibrary]);

  const builtInAD = library.filter((f) => f.builtIn && f.dialect === "ad");
  const builtInOther = library.filter((f) => f.builtIn && f.dialect !== "ad");
  const mine = library.filter((f) => !f.builtIn);

  const group = (title: string, list: app.FilterSummary[]) =>
    list.length > 0 && (
      <>
        <div className="lib-head">
          <span>{title}</span>
          <span className="n">{list.length}</span>
        </div>
        {list.map((f) => (
          <div
            key={f.id}
            className={rowClass(f, editing?.id === f.id)}
            onClick={() => editFilter(f)}
            onDoubleClick={() => f.supported && useFilter(f)}
            title={f.supported ? f.description : f.reason}
            role="button"
            tabIndex={0}
            onKeyDown={(e) => {
              if (e.key === "Enter") editFilter(f);
            }}
          >
            <Icon name="object" className="row-icon" />
            <span className="label">{f.name}</span>
            {f.modified && <span className="dot" title="edited" />}
            <span className={`tag ${f.builtIn ? f.dialect : "mine"}`}>{f.builtIn ? f.dialect : "mine"}</span>
          </div>
        ))}
      </>
    );

  return (
    <>
      <aside className="pane">
        <div className="lib-scroll">
          {group("Built-in · Active Directory", builtInAD)}
          {group("Built-in · any directory", builtInOther)}
          {group("Mine", mine)}
        </div>
      </aside>

      <section className="pane">
        {editing ? <Editor key={editing.id || "new"} filter={editing} /> : <div className="empty">select a filter</div>}
      </section>
    </>
  );
}

function rowClass(f: app.FilterSummary, selected: boolean): string {
  const parts = ["row"];
  if (selected) parts.push("selected");
  if (!f.supported) parts.push("off");
  return parts.join(" ");
}

function Editor({ filter }: { filter: app.FilterSummary }) {
  const check = useStore((s) => s.check);
  const checkFilter = useStore((s) => s.checkFilter);
  const saveFilter = useStore((s) => s.saveFilter);
  const resetFilter = useStore((s) => s.resetFilter);
  const deleteFilter = useStore((s) => s.deleteFilter);
  const useIt = useStore((s) => s.useFilter);

  const [name, setName] = useState(filter.name);
  const [description, setDescription] = useState(filter.description ?? "");
  const [text, setText] = useState(filter.filter);
  const [scope, setScope] = useState(filter.scope || "subtree");
  const [base, setBase] = useState(filter.base ?? "");
  const [columns, setColumns] = useState((filter.columns ?? []).join(", "));

  // Validate as you type. Compiling a filter is nearly free, and the server's
  // answer to a wrong one says nothing about which part was wrong.
  useEffect(() => {
    void checkFilter(text);
  }, [text, checkFilter]);

  const id = filter.id || slug(name);

  return (
    <div className="editor">
      <div className="editor-head">
        <span className="cn">{name || "New filter"}</span>
        {filter.modified && (
          <span className="edited">
            <i /> edited
          </span>
        )}
        <span className="spacer" />
        {filter.supported && filter.id && (
          <button className="ebtn run" onClick={() => useIt(filter)}>
            Run
          </button>
        )}
        {filter.builtIn && filter.modified && (
          <button className="ebtn" onClick={() => void resetFilter(filter.id)}>
            Reset
          </button>
        )}
      </div>

      {!filter.supported && filter.reason && <div className="reason">{filter.reason}</div>}

      <div className="field-group">
        <span className="lbl">Name</span>
        <input className="efield sans" value={name} onChange={(e) => setName(e.target.value)} />
      </div>

      <div className="field-group">
        <span className="lbl">LDAP filter</span>
        <textarea
          className="efield code"
          value={text}
          onChange={(e) => setText(e.target.value)}
          rows={3}
          spellCheck={false}
        />
        <div className={check?.valid ? "check ok" : "check bad"}>
          {check == null
            ? " "
            : check.valid
              ? check.expanded === text
                ? "valid RFC 4515"
                : `valid · expands to ${check.expanded}`
              : check.error}
        </div>
      </div>

      <div className="two">
        <div className="field-group">
          <span className="lbl">Scope</span>
          <div className="segs small">
            {["base", "one", "subtree"].map((v) => (
              <button key={v} className={scope === v ? "seg on" : "seg"} onClick={() => setScope(v)}>
                {v === "one" ? "One level" : v[0].toUpperCase() + v.slice(1)}
              </button>
            ))}
          </div>
        </div>
        <div className="field-group">
          <span className="lbl">Search base</span>
          <input
            className="efield code"
            value={base}
            onChange={(e) => setBase(e.target.value)}
            placeholder="the server's own root"
          />
        </div>
      </div>

      <div className="field-group">
        <span className="lbl">Result columns</span>
        <input
          className="efield code"
          value={columns}
          onChange={(e) => setColumns(e.target.value)}
          placeholder="cn, sAMAccountName, distinguishedName"
        />
      </div>

      <div className="field-group">
        <span className="lbl">Description</span>
        <textarea
          className="efield sans"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={2}
        />
      </div>

      <div className="acts">
        {filter.id && (
          <button className="ebtn danger" onClick={() => void deleteFilter(filter.id)}>
            Delete
          </button>
        )}
        <span className="spacer" />
        <button
          className="ebtn cta"
          disabled={!check?.valid || !name || !id}
          onClick={() =>
            void saveFilter({
              id,
              name,
              description,
              filter: text,
              scope,
              base,
              columns: columns
                .split(",")
                .map((c) => c.trim())
                .filter(Boolean),
              dialect: filter.dialect || "generic",
            } as app.FilterInput)
          }
        >
          Save
        </button>
      </div>
    </div>
  );
}

/** slug turns a name into an ID for a filter the user just created. Nobody
 *  wants to invent a key, and the name is what they already typed. */
function slug(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "");
}
