import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Autosaver, type AutosaveState, type SaveOutcome } from "./autosave";

type Snap = { text: string };
type Call = { snapshot: Snap; expected: number; kind: string; key: string; resolve: (o: SaveOutcome) => void; reject: (e: unknown) => void };

function setup() {
  const calls: Call[] = [];
  const states: AutosaveState[] = [];
  let n = 0;
  let serverVersion = 1;
  const saver = new Autosaver<Snap>(
    {
      save: (snapshot, expected, kind, key) =>
        new Promise<SaveOutcome>((resolve, reject) => calls.push({ snapshot, expected, kind, key, resolve, reject })),
      onState: (s) => states.push(s),
      newKey: () => `key-${++n}`,
    },
    { snapshot: { text: "inicial" }, version: 1 },
  );
  const ok = (c: Call) => c.resolve({ ok: true, version: ++serverVersion, created: true });
  return { saver, calls, states, ok, last: () => states[states.length - 1] };
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("Autosaver", () => {
  it("saves 3 s after the last change, not on every keystroke", async () => {
    const { saver, calls, last } = setup();
    saver.update({ text: "a" });
    await vi.advanceTimersByTimeAsync(2000);
    saver.update({ text: "ab" });
    await vi.advanceTimersByTimeAsync(2999);
    expect(calls).toHaveLength(0);
    expect(last().status).toBe("dirty");
    await vi.advanceTimersByTimeAsync(1);
    expect(calls).toHaveLength(1);
    expect(calls[0]).toMatchObject({ snapshot: { text: "ab" }, expected: 1, kind: "auto" });
    expect(last().status).toBe("saving");
  });

  it("does not save when the change was undone", async () => {
    const { saver, calls, last } = setup();
    saver.update({ text: "x" });
    saver.update({ text: "inicial" });
    await vi.advanceTimersByTimeAsync(20000);
    expect(calls).toHaveLength(0);
    expect(last().status).toBe("saved");
  });

  it("waits at least 15 s between automatic saves", async () => {
    const { saver, calls, ok } = setup();
    saver.update({ text: "1" });
    await vi.advanceTimersByTimeAsync(3000);
    ok(calls[0]);
    await vi.advanceTimersByTimeAsync(0);
    saver.update({ text: "2" });
    // First autosave ran at t=3 s, so the next one cannot run before t=18 s.
    await vi.advanceTimersByTimeAsync(14999); // t≈18 s minus 1 ms: idle long reached, still too soon
    expect(calls).toHaveLength(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(calls).toHaveLength(2);
    expect(calls[1].expected).toBe(2);
  });

  it("keeps edits made during a slow request and saves them next, never marking them saved early", async () => {
    const { saver, calls, ok, last } = setup();
    saver.update({ text: "primero" });
    await vi.advanceTimersByTimeAsync(3000);
    expect(calls).toHaveLength(1);
    saver.update({ text: "primero y más" }); // typed while the request is slow
    await vi.advanceTimersByTimeAsync(10000);
    expect(calls).toHaveLength(1); // one request in flight at most
    ok(calls[0]); // the old response arrives late
    await vi.advanceTimersByTimeAsync(0);
    expect(last().status).toBe("dirty");
    expect(saver.hasUnsavedChanges).toBe(true);
    await vi.advanceTimersByTimeAsync(15000);
    expect(calls).toHaveLength(2);
    expect(calls[1]).toMatchObject({ snapshot: { text: "primero y más" }, expected: 2 });
    ok(calls[1]);
    await vi.advanceTimersByTimeAsync(0);
    expect(last().status).toBe("saved");
    expect(saver.hasUnsavedChanges).toBe(false);
  });

  it("retries network failures with the same idempotency key and expected version", async () => {
    const { saver, calls, ok, states, last } = setup();
    saver.update({ text: "x" });
    await vi.advanceTimersByTimeAsync(3000);
    calls[0].reject(new TypeError("fetch failed"));
    await vi.advanceTimersByTimeAsync(0);
    expect(states.some((s) => s.retrying)).toBe(true);
    await vi.advanceTimersByTimeAsync(1000);
    expect(calls).toHaveLength(2);
    calls[1].resolve({ ok: false, kind: "server", message: "502" });
    await vi.advanceTimersByTimeAsync(3000);
    expect(calls).toHaveLength(3);
    expect(new Set(calls.map((c) => c.key)).size).toBe(1);
    expect(new Set(calls.map((c) => c.expected)).size).toBe(1);
    ok(calls[2]);
    await vi.advanceTimersByTimeAsync(0);
    expect(last().status).toBe("saved");
  });

  it("stops on conflict, keeps the local buffer and never retries with the other version", async () => {
    const { saver, calls, last } = setup();
    saver.update({ text: "mi versión" });
    await vi.advanceTimersByTimeAsync(3000);
    calls[0].resolve({ ok: false, kind: "conflict", currentVersion: 5 });
    await vi.advanceTimersByTimeAsync(0);
    expect(last()).toMatchObject({ status: "conflict", conflictVersion: 5, version: 1 });
    saver.update({ text: "mi versión, seguí escribiendo" });
    await saver.saveNow();
    await vi.advanceTimersByTimeAsync(60000);
    expect(calls).toHaveLength(1);
    expect(saver.snapshot).toEqual({ text: "mi versión, seguí escribiendo" });
    // Only an explicit reset (after the owner confirms discarding) adopts the server version.
    saver.reset({ text: "versión del servidor" }, 5);
    expect(last()).toMatchObject({ status: "saved", version: 5 });
  });

  it("serializes a manual save behind the request in flight", async () => {
    const { saver, calls, ok, last } = setup();
    saver.update({ text: "auto" });
    await vi.advanceTimersByTimeAsync(3000);
    saver.update({ text: "auto + manual" });
    const manual = saver.saveNow();
    await vi.advanceTimersByTimeAsync(0);
    expect(calls).toHaveLength(1);
    ok(calls[0]);
    await vi.advanceTimersByTimeAsync(0);
    expect(calls).toHaveLength(2);
    expect(calls[1]).toMatchObject({ kind: "manual", expected: 2, snapshot: { text: "auto + manual" } });
    ok(calls[1]);
    await manual;
    await vi.advanceTimersByTimeAsync(0);
    expect(last().status).toBe("saved");
  });

  it("reports validation errors without looping, and tries again after the next edit", async () => {
    const { saver, calls, last } = setup();
    saver.update({ text: "malo" });
    await vi.advanceTimersByTimeAsync(3000);
    calls[0].resolve({ ok: false, kind: "invalid", message: "slug inválido", fields: { slug: "x" } });
    await vi.advanceTimersByTimeAsync(60000);
    expect(calls).toHaveLength(1);
    expect(last()).toMatchObject({ status: "error", fields: { slug: "x" } });
    saver.update({ text: "bueno" });
    await vi.advanceTimersByTimeAsync(15000);
    expect(calls).toHaveLength(2);
  });
});
