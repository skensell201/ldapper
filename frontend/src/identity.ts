/** shortIdentity turns a bound identity into something that fits in a pill.
 *
 *  A service account's distinguished name runs to sixty characters and gets
 *  cut off mid-word, which tells nobody anything. The leftmost value is the
 *  part a person recognises; the whole name stays in the tooltip.
 */
export function shortIdentity(boundAs: string): string {
  if (!boundAs) return "anonymous";

  // A down-level logon name is already short: CORP\a.kensel. It is told apart
  // from a distinguished name by having no equals sign — a DN can contain a
  // backslash too, as the escape before a comma inside a name.
  if (boundAs.includes("\\") && !boundAs.includes("=")) return boundAs;

  const comma = firstUnescapedComma(boundAs);
  const rdn = comma < 0 ? boundAs : boundAs.slice(0, comma);
  const equals = rdn.indexOf("=");
  return equals < 0 ? rdn : unescape(rdn.slice(equals + 1));
}

function firstUnescapedComma(dn: string): number {
  for (let i = 0; i < dn.length; i++) {
    if (dn[i] === "\\") {
      i++;
      continue;
    }
    if (dn[i] === ",") return i;
  }
  return -1;
}

/** unescape resolves the two forms RFC 4514 allows, so a comma inside a name
 *  reads as a comma rather than as \2C. */
function unescape(s: string): string {
  if (!s.includes("\\")) return s;

  let out = "";
  for (let i = 0; i < s.length; i++) {
    if (s[i] !== "\\" || i + 1 >= s.length) {
      out += s[i];
      continue;
    }
    const hex = s.slice(i + 1, i + 3);
    if (/^[0-9a-fA-F]{2}$/.test(hex)) {
      out += String.fromCharCode(parseInt(hex, 16));
      i += 2;
      continue;
    }
    out += s[i + 1];
    i++;
  }
  return out;
}
