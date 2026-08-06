/** The Go side, or a stand-in for it.
 *
 *  The window injects `window.go` before the bundle runs, so the generated
 *  bindings work. Under the Vite dev server nothing injects it, and the
 *  interface would fail on its first call — so a mock takes over there,
 *  letting the interface be worked on and photographed without a Go build.
 *
 *  Everything imports from here rather than from wailsjs directly, so there
 *  is exactly one place that decides which is in use.
 */
import * as bindings from "../wailsjs/go/app/App";
import * as runtime from "../wailsjs/runtime/runtime";
import { mock, mockRuntime, usingMock } from "./dev/mock";

type Bindings = typeof bindings;

export const api: Bindings = (usingMock ? (mock as unknown as Bindings) : bindings);
export const EventsOn = usingMock ? mockRuntime.EventsOn : runtime.EventsOn;
export const Environment = usingMock ? mockRuntime.Environment : runtime.Environment;
