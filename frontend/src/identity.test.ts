import { describe, expect, it } from "vitest";
import { rdnValue, shortIdentity } from "./identity";

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

describe("the readable part of a relative name", () => {
  it("drops the attribute name", () => {
    expect(rdnValue("cn=Anna Volkova")).toBe("Anna Volkova");
  });

  // The tree and the attribute pane both show this, and they disagreed: the
  // tree unescaped it, the heading above the attributes did not.
  it("undoes escaping in both forms", () => {
    expect(rdnValue("cn=Volkova\\2C Anna")).toBe("Volkova, Anna");
    expect(rdnValue("cn=Volkova\\, Anna")).toBe("Volkova, Anna");
  });

  it("leaves something without an attribute name alone", () => {
    expect(rdnValue("Anna")).toBe("Anna");
  });
});
