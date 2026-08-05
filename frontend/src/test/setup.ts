import "@testing-library/jest-dom/vitest";
import { vi } from "vitest";

// jsdom gives every element a zero size, and @tanstack/react-virtual asks the
// scroll container how tall it is before deciding what to render. Without a
// size it renders nothing, and a test asserting on rows would be asserting on
// an empty list.
Object.defineProperty(HTMLElement.prototype, "clientHeight", { value: 800, configurable: true });
Object.defineProperty(HTMLElement.prototype, "clientWidth", { value: 900, configurable: true });
Object.defineProperty(HTMLElement.prototype, "offsetHeight", { value: 800, configurable: true });
Object.defineProperty(HTMLElement.prototype, "offsetWidth", { value: 900, configurable: true });

HTMLElement.prototype.getBoundingClientRect = function () {
  return {
    width: 900, height: 800, top: 0, left: 0, bottom: 800, right: 900, x: 0, y: 0,
    toJSON: () => ({}),
  } as DOMRect;
};

// jsdom has no ResizeObserver, which the virtualiser subscribes to.
globalThis.ResizeObserver = class {
  observe() {}
  unobserve() {}
  disconnect() {}
} as unknown as typeof ResizeObserver;

// The clipboard is used by Copy DN.
Object.assign(navigator, { clipboard: { writeText: vi.fn() } });
