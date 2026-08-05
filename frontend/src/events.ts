/** Payloads of the events Go emits.
 *
 *  These are declared by hand rather than generated. Wails only produces
 *  TypeScript for types that appear in a bound method's signature, and these
 *  travel as events — the generator never sees them.
 *
 *  Drift is caught by TestEventPayloadsMatchTheirTypeScript in app/events_test.go,
 *  which asserts the exact JSON keys Go emits. Change a field here and you
 *  must change it there.
 */

export const EVENT_SEARCH_BATCH = "search:batch";
export const EVENT_SEARCH_DONE = "search:done";

export interface SearchRow {
  dn: string;
  icon: string;
  cells: string[];
}

export interface SearchBatch {
  rows: SearchRow[];
  matched: number;
}

export interface SearchDone {
  matched: number;
  truncated: boolean;
  reason?: string;
  error?: string;
  cancelled: boolean;
  columns: string[];
  elapsed: number;
}
