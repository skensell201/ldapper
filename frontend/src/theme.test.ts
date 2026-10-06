import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { apply, loadPref, resolve, savePref } from "./theme";

function systemIsDark(dark: boolean) {
  vi.stubGlobal("matchMedia", (q: string) => ({
    matches: dark && q.includes("dark"),
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
}

// Node ships a localStorage of its own that shadows jsdom's and does nothing
// without a backing file, so each test gets a plain in-memory one.
beforeEach(() => {
  const data = new Map<string, string>();
  vi.stubGlobal("localStorage", {
    getItem: (k: string) => data.get(k) ?? null,
    setItem: (k: string, v: string) => void data.set(k, v),
    removeItem: (k: string) => void data.delete(k),
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("theme preference", () => {
  it("defaults to following the system", () => {
    expect(loadPref()).toBe("system");
  });

  it("remembers a pinned theme", () => {
    savePref("dark");
    expect(loadPref()).toBe("dark");
  });

  it("forgets the pin when set back to system", () => {
    savePref("light");
    savePref("system");
    expect(localStorage.getItem("ldapper.theme")).toBeNull();
    expect(loadPref()).toBe("system");
  });

  it("ignores a stored value it does not know", () => {
    localStorage.setItem("ldapper.theme", "sepia");
    expect(loadPref()).toBe("system");
  });
});

describe("resolve", () => {
  it("follows the system when asked to", () => {
    systemIsDark(true);
    expect(resolve("system")).toBe("dark");
    systemIsDark(false);
    expect(resolve("system")).toBe("light");
  });

  it("keeps a pinned theme whatever the system says", () => {
    systemIsDark(true);
    expect(resolve("light")).toBe("light");
  });
});

describe("apply", () => {
  it("puts the theme where the stylesheet reads it", () => {
    apply("dark");
    expect(document.documentElement.dataset.theme).toBe("dark");
  });
});
