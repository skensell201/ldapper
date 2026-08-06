import { describe, expect, it } from "vitest";
import { healthLabel, healthOf } from "./health";

const connection = {
  profileId: "dev", connected: true, host: "dc3-spb.da.lan",
  boundAs: "CN=SpaceReader,DC=da,DC=lan", rootDN: "DC=da,DC=lan",
  isActiveDirectory: true, supportsPaging: true, dialects: ["generic", "ad"],
  encryption: "ldaps",
} as never;

describe("what colour a connection is", () => {
  it("is green when it is encrypted and working", () => {
    expect(healthOf(connection, "")).toBe("good");
    expect(healthLabel("good", connection, "")).toBe("Connected");
  });

  // Credentials in the clear are worth knowing about even when everything
  // works, which is exactly what an amber state is for.
  it("is amber when there is no TLS", () => {
    const plain = { ...(connection as object), encryption: "none" } as never;
    expect(healthOf(plain, "")).toBe("warn");
    expect(healthLabel("warn", plain, "")).toMatch(/unencrypted/);
  });

  it("is red when the server could not be reached", () => {
    expect(healthOf(null, "dial tcp: connection refused")).toBe("bad");
    expect(healthLabel("bad", null, "dial tcp: connection refused")).toBe("dial tcp: connection refused");
  });

  // An error outranks a working connection: something has gone wrong since,
  // and saying "Connected" over the top of it would be a lie.
  it("is red even while a connection is open", () => {
    expect(healthOf(connection, "The server stopped early.")).toBe("bad");
  });

  it("is grey before anything has been tried", () => {
    expect(healthOf(null, "")).toBe("idle");
    expect(healthLabel("idle", null, "")).toBe("Not connected");
  });

  it("says disconnected once a connection has existed", () => {
    const closed = { ...(connection as object), connected: false } as never;
    expect(healthOf(closed, "")).toBe("idle");
    expect(healthLabel("idle", closed, "")).toBe("Disconnected");
  });
});
