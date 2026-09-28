import { afterEach, describe, expect, it, vi } from "vitest";
import { loginState } from "./login.server";

type Call = { path: string; cookie: string | null };

// Fake internal API: per-path handlers; a thrown error simulates a network failure.
function fakeApi(routes: Record<string, () => Response>) {
  const calls: Call[] = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: URL | string, init?: RequestInit) => {
      const url = new URL(input);
      calls.push({ path: url.pathname, cookie: new Headers(init?.headers).get("Cookie") });
      const handler = routes[url.pathname];
      if (!handler) throw new Error(`unexpected ${url.pathname}`);
      return handler();
    }),
  );
  return calls;
}

const json = (body: unknown, status = 200) => () => Response.json(body, { status });
const networkError = () => {
  throw new TypeError("fetch failed");
};

function request(cookie?: string, query = "") {
  return new Request(`http://localhost:8000/admin/login${query}`, {
    headers: cookie ? { Cookie: cookie } : {},
  });
}

afterEach(() => vi.unstubAllGlobals());

describe("loginState without a session cookie", () => {
  it("offers login when OAuth is configured", async () => {
    const calls = fakeApi({ "/api/v1/auth/status": json({ login_enabled: true }) });
    expect(await loginState(request())).toEqual({ kind: "page", availability: "enabled", returnTo: "/admin" });
    // No session to check: only the availability endpoint is called, without cookies.
    expect(calls).toEqual([{ path: "/api/v1/auth/status", cookie: null }]);
  });

  it("reports not configured when OAuth has no credentials", async () => {
    fakeApi({ "/api/v1/auth/status": json({ login_enabled: false }) });
    expect(await loginState(request())).toMatchObject({ availability: "not_configured" });
  });

  it("distinguishes a network error from missing configuration", async () => {
    fakeApi({ "/api/v1/auth/status": networkError });
    expect(await loginState(request())).toMatchObject({ availability: "unavailable" });
  });

  it("treats unexpected answers as unavailable", async () => {
    fakeApi({ "/api/v1/auth/status": json({ code: "internal" }, 500) });
    expect(await loginState(request())).toMatchObject({ availability: "unavailable" });
    fakeApi({ "/api/v1/auth/status": json({ login_enabled: "yes" }) });
    expect(await loginState(request())).toMatchObject({ availability: "unavailable" });
  });

  it("does not forward unrelated cookies to the availability check", async () => {
    const calls = fakeApi({ "/api/v1/auth/status": json({ login_enabled: true }) });
    await loginState(request("_ga=GA1.1; theme=dark"));
    expect(calls).toEqual([{ path: "/api/v1/auth/status", cookie: null }]);
  });
});

describe("loginState with a session cookie", () => {
  it("sends a valid owner to the requested admin page", async () => {
    const calls = fakeApi({
      "/api/v1/auth/me": json({ github_user_id: 1, github_login: "owner", expires_at: "", csrf_token: "t" }),
    });
    expect(await loginState(request("bl_session=abc; _ga=1", "?return_to=%2Fadmin%2Fposts"))).toEqual({
      kind: "redirect",
      to: "/admin/posts",
    });
    expect(calls).toEqual([{ path: "/api/v1/auth/me", cookie: "bl_session=abc" }]);
  });

  it("shows the login page when the session is no longer valid", async () => {
    fakeApi({
      "/api/v1/auth/me": json({ code: "unauthenticated" }, 401),
      "/api/v1/auth/status": json({ login_enabled: true }),
    });
    expect(await loginState(request("bl_session=expired"))).toMatchObject({ kind: "page", availability: "enabled" });
  });

  it("reports unavailable when the API is down", async () => {
    fakeApi({ "/api/v1/auth/me": networkError, "/api/v1/auth/status": networkError });
    expect(await loginState(request("bl_session=abc"))).toMatchObject({ kind: "page", availability: "unavailable" });
  });

  it("never redirects outside the admin area", async () => {
    fakeApi({ "/api/v1/auth/me": json({ github_user_id: 1, github_login: "o", expires_at: "", csrf_token: "t" }) });
    for (const target of ["https://evil.example", "//evil.example", "/es", "/administrator"]) {
      expect(await loginState(request("bl_session=abc", `?return_to=${encodeURIComponent(target)}`))).toEqual({
        kind: "redirect",
        to: "/admin",
      });
    }
  });
});
