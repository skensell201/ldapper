/** What a decoded value is, so its chip can be toned by kind: an identifier
 *  (a SID, a GUID), a moment in time, or a flag packed into a bit field.
 *
 *  The engine hands the interface finished strings, so the kind is read back
 *  from their shape. The shapes are the ones internal/decode produces. */
export type DecodedKind = "id" | "time" | "flag";

const TIME = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} UTC$/;
const FLAG = /^[A-Z][A-Z0-9_]+$/;

export function decodedKind(value: string): DecodedKind {
  if (TIME.test(value) || value === "never" || value === "not set") return "time";
  if (FLAG.test(value)) return "flag";
  return "id";
}
