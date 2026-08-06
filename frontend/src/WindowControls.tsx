import { useEffect, useState } from "react";
import { Quit, WindowIsMaximised, WindowMinimise, WindowToggleMaximise } from "./api";
import "./WindowControls.css";

/** The minimise, maximise and close buttons, drawn by us.
 *
 *  Windows is frameless, so nothing else draws them. macOS is not: its
 *  controls belong to the system and already sit inside this bar, so this
 *  component is never rendered there.
 */
export function WindowControls() {
  const [maximised, setMaximised] = useState(false);

  // The window can be maximised by dragging it to the top edge as well as by
  // this button, so the icon has to follow the window rather than a click.
  useEffect(() => {
    const check = () => void WindowIsMaximised().then(setMaximised);
    check();
    window.addEventListener("resize", check);
    return () => window.removeEventListener("resize", check);
  }, []);

  return (
    <div className="winctl no-drag">
      <button onClick={() => WindowMinimise()} aria-label="Minimise" title="Minimise">
        <svg viewBox="0 0 10 10" aria-hidden="true">
          <path d="M0 5h10" />
        </svg>
      </button>

      <button
        onClick={() => WindowToggleMaximise()}
        aria-label={maximised ? "Restore" : "Maximise"}
        title={maximised ? "Restore" : "Maximise"}
      >
        {maximised ? (
          <svg viewBox="0 0 10 10" aria-hidden="true">
            <path d="M2.5 2.5V0.5h7v7h-2" />
            <rect x="0.5" y="2.5" width="7" height="7" />
          </svg>
        ) : (
          <svg viewBox="0 0 10 10" aria-hidden="true">
            <rect x="0.5" y="0.5" width="9" height="9" />
          </svg>
        )}
      </button>

      <button className="close" onClick={() => Quit()} aria-label="Close" title="Close">
        <svg viewBox="0 0 10 10" aria-hidden="true">
          <path d="M0.5 0.5l9 9M9.5 0.5l-9 9" />
        </svg>
      </button>
    </div>
  );
}
