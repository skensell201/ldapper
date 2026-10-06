import { useEffect, useState } from "react";

/** Which theme the window wears. "system" follows the operating system and
 *  keeps following it; the other two pin a theme regardless.
 *
 *  The choice is a preference of this machine, not of a connection, so it
 *  lives in the webview's own storage rather than in the settings files the
 *  engine writes. Storage can be unavailable; the window then simply follows
 *  the system, which is the default anyway. */
export type ThemePref = "system" | "light" | "dark";
export type Theme = "light" | "dark";

const KEY = "ldapper.theme";
const DARK = "(prefers-color-scheme: dark)";

export function loadPref(): ThemePref {
  try {
    const v = localStorage.getItem(KEY);
    if (v === "light" || v === "dark" || v === "system") return v;
  } catch {
    // Storage refused: fall through to the default.
  }
  return "system";
}

export function savePref(pref: ThemePref): void {
  try {
    if (pref === "system") localStorage.removeItem(KEY);
    else localStorage.setItem(KEY, pref);
  } catch {
    // Not remembered, but still applied for this session.
  }
}

function systemTheme(): Theme {
  return typeof matchMedia === "function" && matchMedia(DARK).matches ? "dark" : "light";
}

export function resolve(pref: ThemePref): Theme {
  return pref === "system" ? systemTheme() : pref;
}

/** Puts the theme on the document. tokens.css reads it from data-theme. */
export function apply(theme: Theme): void {
  document.documentElement.dataset.theme = theme;
}

/** The preference, a setter that remembers it, and — while the preference is
 *  "system" — a listener that follows the operating system as it changes. */
export function useTheme(): [ThemePref, (pref: ThemePref) => void] {
  const [pref, setPref] = useState<ThemePref>(loadPref);

  useEffect(() => {
    apply(resolve(pref));
    if (pref !== "system" || typeof matchMedia !== "function") return;
    const mq = matchMedia(DARK);
    const follow = () => apply(resolve("system"));
    mq.addEventListener("change", follow);
    return () => mq.removeEventListener("change", follow);
  }, [pref]);

  return [
    pref,
    (next) => {
      savePref(next);
      setPref(next);
    },
  ];
}
