import { describe, expect, it } from "vitest";
import { shortIdentity } from "./identity";

describe("the identity shown in the chrome", () => {
  it("keeps only the leftmost value of a distinguished name", () => {
    // The full name ran off the end of the pill and was cut mid-word.
    expect(shortIdentity("CN=SpaceReader,OU=Service Accounts,DC=da,DC=lan")).toBe("SpaceReader");
  });

  it("leaves a down-level logon name alone", () => {
    expect(shortIdentity("CORP\\a.kensel")).toBe("CORP\\a.kensel");
  });

  it("says anonymous when nothing is bound", () => {
    expect(shortIdentity("")).toBe("anonymous");
  });

  it("undoes the escaping around a comma in a name", () => {
    expect(shortIdentity("CN=Volkova\\2C Anna,OU=Users,DC=example,DC=com")).toBe("Volkova, Anna");
    expect(shortIdentity("CN=Volkova\\, Anna,OU=Users,DC=example,DC=com")).toBe("Volkova, Anna");
  });

  it("passes through something that is not a distinguished name", () => {
    expect(shortIdentity("admin")).toBe("admin");
  });

  it("handles a single-component name", () => {
    expect(shortIdentity("cn=admin")).toBe("admin");
  });
});
