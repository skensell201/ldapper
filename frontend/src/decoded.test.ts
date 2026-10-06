import { describe, expect, it } from "vitest";
import { decodedKind } from "./decoded";

describe("decodedKind", () => {
  it.each([
    ["S-1-5-21-3638186439-3476304828-32309524-1104", "id"],
    ["3f2504e0-4f89-11d3-9a0c-0305e82c3301", "id"],
    ["2025-04-18 00:24:21 UTC", "time"],
    ["never", "time"],
    ["not set", "time"],
    ["NORMAL_ACCOUNT", "flag"],
    ["DONT_EXPIRE_PASSWORD", "flag"],
  ])("reads %s as %s", (value, kind) => {
    expect(decodedKind(value)).toBe(kind);
  });
});
