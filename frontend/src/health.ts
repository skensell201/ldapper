import type { app } from "../wailsjs/go/models";

export type Health = "good" | "warn" | "bad" | "idle";

/** healthOf reduces everything known about a connection to the one thing the
 *  chrome and the status bar both display.
 *
 *  Both read it, so they cannot disagree about what colour the dot should be. */
export function healthOf(connection: app.ConnectionState | null, error: string): Health {
  if (error) return "bad";
  if (!connection?.connected) return "idle";
  // A connection carrying credentials in the clear is working, and is still
  // something you should know about.
  if (connection.encryption === "none") return "warn";
  return "good";
}

/** healthLabel is what the status bar writes beside the dot. */
export function healthLabel(health: Health, connection: app.ConnectionState | null, error: string): string {
  switch (health) {
    case "bad":
      return error;
    case "warn":
      return "Connected, unencrypted";
    case "good":
      return "Connected";
    default:
      return connection ? "Disconnected" : "Not connected";
  }
}
