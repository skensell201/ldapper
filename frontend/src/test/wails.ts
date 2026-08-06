import { vi } from "vitest";

/** Everything the store calls on the Go side, replaced with spies.
 *
 *  The real bindings are generated into wailsjs/ and only exist inside a
 *  running window, so importing the store in a test would fail without this. */
export const api = {
  ListProfiles: vi.fn(async () => []),
  ListFilters: vi.fn(async () => []),
  Connect: vi.fn(),
  Disconnect: vi.fn(async () => {}),
  TrustCertificate: vi.fn(async () => ""),
  Children: vi.fn(),
  Entry: vi.fn(),
  SaveProfile: vi.fn(async () => ""),
  DeleteProfile: vi.fn(async () => ""),
  StartSearch: vi.fn(),
  StopSearch: vi.fn(async () => {}),
  ValidateFilter: vi.fn(async () => ({ valid: true, expanded: "", error: "" })),
  SaveFilter: vi.fn(async () => ""),
  ResetFilter: vi.fn(async () => ""),
  DeleteFilter: vi.fn(async () => ""),
  RestoreDefaultFilters: vi.fn(async () => ""),
  ChooseExportPath: vi.fn(async () => ""),
  ExportSearch: vi.fn(async () => ""),
  ExportEntry: vi.fn(async () => ""),
};

/** Handlers the store registered with EventsOn, so a test can emit an event
 *  the way Go would. */
export const handlers: Record<string, (data: unknown) => void> = {};

vi.mock("../../wailsjs/go/app/App", () => api);
vi.mock("../../wailsjs/runtime/runtime", () => ({
  EventsOn: (name: string, fn: (data: unknown) => void) => {
    handlers[name] = fn;
    return () => delete handlers[name];
  },
  Environment: async () => ({ buildType: "test", platform: "darwin", arch: "arm64" }),
}));
