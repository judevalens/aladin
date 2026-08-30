// @vitest-environment node
import { readFileSync } from "node:fs";
import { runInNewContext } from "node:vm";
import { describe, expect, it } from "vitest";

const source = readFileSync(new URL("../../src-tauri/src/local_frontend_storage.js", import.meta.url), "utf8");
const migration = runInNewContext(`${source}\nstorageMigration;`) as {
  marker: string;
  collect(storage: Storage): Record<string, string>;
  restore(storage: Storage, entries: Record<string, unknown>): void;
};

function storage(values: Record<string, string> = {}): Storage {
  const data = new Map(Object.entries(values));
  return {
    get length() { return data.size; },
    key: (index) => [...data.keys()][index] ?? null,
    getItem: (key) => data.get(key) ?? null,
    setItem: (key, value) => { data.set(key, value); },
    removeItem: (key) => { data.delete(key); },
    clear: () => data.clear(),
  };
}

describe("bundled frontend storage migration", () => {
  it("transfers only app-owned values without modifying the legacy origin", () => {
    const legacy = storage({
      "aladin.desktop_session": JSON.stringify({ token: "test-token" }),
      "aladin.theme": "light",
      "other-app": "unrelated",
      [migration.marker]: "1",
    });
    const entries = migration.collect(legacy);
    expect(Object.keys(entries).sort()).toEqual(["aladin.desktop_session", "aladin.theme"]);
    const target = storage();
    migration.restore(target, entries);
    expect(target.getItem("aladin.desktop_session")).toBe(legacy.getItem("aladin.desktop_session"));
    expect(target.getItem("aladin.theme")).toBe("light");
    expect(target.getItem("other-app")).toBeNull();
    expect(legacy.length).toBe(4);
  });

  it("does not resurrect a logged-out session on later launches", () => {
    const target = storage();
    const entries = { "aladin.desktop_session": "old-session" };
    migration.restore(target, entries);
    target.removeItem("aladin.desktop_session");
    migration.restore(target, entries);
    expect(target.getItem("aladin.desktop_session")).toBeNull();
  });

  it("preserves newer preferences and ignores invalid entries", () => {
    const target = storage({ "aladin.theme": "dark" });
    migration.restore(target, { "aladin.theme": "light", "aladin.bad": null, foreign: "no" });
    expect(target.getItem("aladin.theme")).toBe("dark");
    expect(target.getItem("aladin.bad")).toBeNull();
    expect(target.getItem("foreign")).toBeNull();
  });

  it("does not mark an interrupted transfer complete", () => {
    const target = storage();
    const setItem = target.setItem;
    target.setItem = (key, value) => {
      if (key === "aladin.theme") throw new Error("quota exceeded");
      setItem(key, value);
    };
    const entries = { "aladin.desktop_session": "test-session", "aladin.theme": "light" };
    expect(() => migration.restore(target, entries)).toThrow("quota exceeded");
    expect(target.getItem(migration.marker)).toBeNull();
    target.setItem = setItem;
    migration.restore(target, entries);
    expect(target.getItem("aladin.theme")).toBe("light");
    expect(target.getItem(migration.marker)).toBe("1");
  });
});
