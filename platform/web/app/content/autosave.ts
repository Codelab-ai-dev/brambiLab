// Autosave state machine (web-v1.md §6.2). Framework-free so it can be tested with fake timers.
//
// - Auto: after 3 s idle, only if the snapshot changed, at most once every 15 s, one request in
//   flight. Changes made during a request stay pending; a late response never marks newer edits saved.
// - Manual ("Guardar revisión"): immediately, queued behind a request in flight.
// - Network/server failures retry with the same Idempotency-Key and expected_version.
// - 409: stop autosaving, keep the local buffer, report the server version. Never retry with it.

export type SaveKind = "manual" | "auto";

export type SaveOutcome =
  | { ok: true; version: number; created: boolean }
  | { ok: false; kind: "conflict"; currentVersion: number }
  | { ok: false; kind: "invalid"; message: string; fields?: Record<string, string> }
  | { ok: false; kind: "unauthenticated" }
  | { ok: false; kind: "network" | "server"; message: string };

export type AutosaveStatus = "saved" | "dirty" | "saving" | "error" | "conflict";

export type AutosaveState = {
  status: AutosaveStatus;
  /** Server version the next save must expect. */
  version: number;
  message?: string;
  fields?: Record<string, string>;
  conflictVersion?: number;
  lastSavedAt?: number;
  retrying?: boolean;
};

type Timer = unknown;

export type AutosaveDeps<S> = {
  save: (snapshot: S, expectedVersion: number, kind: SaveKind, idempotencyKey: string) => Promise<SaveOutcome>;
  onState: (state: AutosaveState) => void;
  newKey: () => string;
  now?: () => number;
  setTimer?: (fn: () => void, ms: number) => Timer;
  clearTimer?: (t: Timer) => void;
  idleMs?: number;
  minAutoIntervalMs?: number;
  retryDelaysMs?: number[];
};

export function stableStringify(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(stableStringify).join(",")}]`;
  if (value && typeof value === "object") {
    const entries = Object.entries(value as Record<string, unknown>)
      .filter(([, v]) => v !== undefined)
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
    return `{${entries.map(([k, v]) => `${JSON.stringify(k)}:${stableStringify(v)}`).join(",")}}`;
  }
  return JSON.stringify(value);
}

export class Autosaver<S> {
  private deps: Required<AutosaveDeps<S>>;
  private current: S;
  private currentJSON: string;
  private savedJSON: string;
  private state: AutosaveState;
  private inFlight: Promise<void> | null = null;
  private manualQueued = false;
  private idleTimer: Timer | null = null;
  private lastAutoAt = Number.NEGATIVE_INFINITY;
  private disposed = false;

  constructor(deps: AutosaveDeps<S>, initial: { snapshot: S; version: number }) {
    this.deps = {
      now: () => Date.now(),
      setTimer: (fn, ms) => setTimeout(fn, ms),
      clearTimer: (t) => clearTimeout(t as ReturnType<typeof setTimeout>),
      idleMs: 3000,
      minAutoIntervalMs: 15000,
      retryDelaysMs: [1000, 3000, 9000],
      ...deps,
    };
    this.current = initial.snapshot;
    this.currentJSON = this.savedJSON = stableStringify(initial.snapshot);
    this.state = { status: "saved", version: initial.version };
  }

  get snapshot(): S {
    return this.current;
  }

  get status(): AutosaveState {
    return this.state;
  }

  /** True when something typed is not persisted yet (warn before leaving). */
  get hasUnsavedChanges(): boolean {
    return this.currentJSON !== this.savedJSON;
  }

  update(snapshot: S): void {
    if (this.disposed) return;
    this.current = snapshot;
    this.currentJSON = stableStringify(snapshot);
    if (this.state.status === "conflict") return; // keep the buffer, never autosave over a conflict
    if (!this.inFlight) {
      this.set(this.hasUnsavedChanges ? { status: "dirty", message: undefined, fields: undefined } : { status: "saved" });
    }
    this.scheduleIdle();
  }

  /** "Guardar revisión": persists now, or right after the request in flight. */
  async saveNow(): Promise<void> {
    if (this.disposed || this.state.status === "conflict") return;
    this.clearIdle();
    if (this.inFlight) {
      this.manualQueued = true;
      await this.inFlight;
      return this.inFlight ?? undefined;
    }
    if (!this.hasUnsavedChanges) {
      this.set({ status: "saved" });
      return;
    }
    return this.run("manual");
  }

  /** After a conflict: adopt the server's snapshot and version, discarding the local buffer. */
  reset(snapshot: S, version: number): void {
    this.clearIdle();
    this.current = snapshot;
    this.currentJSON = this.savedJSON = stableStringify(snapshot);
    this.state = { status: "saved", version };
    this.deps.onState(this.state);
  }

  dispose(): void {
    this.disposed = true;
    this.clearIdle();
  }

  private set(patch: Partial<AutosaveState>) {
    this.state = { ...this.state, ...patch };
    if (!this.disposed) this.deps.onState(this.state);
  }

  private clearIdle() {
    if (this.idleTimer !== null) this.deps.clearTimer(this.idleTimer);
    this.idleTimer = null;
  }

  private scheduleIdle(delay = this.deps.idleMs) {
    this.clearIdle();
    if (!this.hasUnsavedChanges) return;
    this.idleTimer = this.deps.setTimer(() => this.onIdle(), delay);
  }

  private onIdle() {
    this.idleTimer = null;
    if (this.inFlight || this.disposed || this.state.status === "conflict" || !this.hasUnsavedChanges) return;
    const wait = this.lastAutoAt + this.deps.minAutoIntervalMs - this.deps.now();
    if (wait > 0) {
      this.idleTimer = this.deps.setTimer(() => this.onIdle(), wait);
      return;
    }
    void this.run("auto");
  }

  private run(kind: SaveKind): Promise<void> {
    const snapshot = this.current;
    const json = this.currentJSON;
    const expected = this.state.version;
    const key = this.deps.newKey();
    if (kind === "auto") this.lastAutoAt = this.deps.now();
    this.set({ status: "saving", retrying: false, message: undefined, fields: undefined });

    const attempt = async (): Promise<void> => {
      const delays = this.deps.retryDelaysMs;
      for (let i = 0; ; i++) {
        let outcome: SaveOutcome;
        try {
          outcome = await this.deps.save(snapshot, expected, kind, key);
        } catch (err) {
          outcome = { ok: false, kind: "network", message: err instanceof Error ? err.message : "Error de red" };
        }
        if (this.disposed) return;
        if (!outcome.ok && (outcome.kind === "network" || outcome.kind === "server") && i < delays.length) {
          this.set({ retrying: true, message: outcome.message });
          await new Promise<void>((resolve) => this.deps.setTimer(resolve, delays[i]));
          continue;
        }
        this.finish(outcome, json);
        return;
      }
    };

    const p = attempt().finally(() => {
      if (this.inFlight === p) this.inFlight = null;
      this.afterRequest();
    });
    this.inFlight = p;
    return p;
  }

  private finish(outcome: SaveOutcome, sentJSON: string) {
    if (outcome.ok) {
      this.savedJSON = sentJSON;
      this.set({
        version: outcome.version,
        status: this.hasUnsavedChanges ? "dirty" : "saved",
        lastSavedAt: this.deps.now(),
        retrying: false,
        message: undefined,
      });
      return;
    }
    switch (outcome.kind) {
      case "conflict":
        this.clearIdle();
        this.manualQueued = false;
        this.set({ status: "conflict", conflictVersion: outcome.currentVersion, retrying: false });
        return;
      case "invalid":
        this.set({ status: "error", message: outcome.message, fields: outcome.fields, retrying: false });
        return;
      case "unauthenticated":
        this.set({ status: "error", message: "La sesión caducó. Inicia sesión de nuevo en otra pestaña y vuelve a guardar.", retrying: false });
        return;
      default:
        this.set({ status: "error", message: outcome.message, retrying: false });
    }
  }

  private afterRequest() {
    if (this.disposed || this.state.status === "conflict") return;
    if (this.manualQueued) {
      this.manualQueued = false;
      if (this.hasUnsavedChanges) void this.run("manual");
      return;
    }
    // Validation/auth errors wait for the next edit or a manual save instead of looping.
    if (this.state.status === "error") return;
    if (this.hasUnsavedChanges) this.scheduleIdle();
  }
}
