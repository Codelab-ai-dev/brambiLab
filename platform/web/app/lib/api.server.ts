// Server-only client for the Go API. The web never decides authentication or authorization:
// it forwards the owner's session cookie to the trusted internal API and renders the answer
// (web-v1.md §3, §11).

const internalApiUrl = process.env.INTERNAL_API_URL ?? "http://localhost:8080";

// The API names the cookie with the __Host- prefix on https and without it on localhost.
const sessionCookieNames = new Set(["__Host-bl_session", "bl_session"]);

export type Owner = {
  github_user_id: number;
  github_login: string;
  expires_at: string;
  csrf_token: string;
};

export type OwnerResult =
  | { status: "authenticated"; owner: Owner }
  | { status: "anonymous" }
  | { status: "not_configured" };

/**
 * Keeps only the session cookie from the browser's Cookie header. Other cookies (analytics,
 * third parties, future features) never leave the web process.
 */
export function sessionCookieHeader(cookieHeader: string | null): string | null {
  if (!cookieHeader) return null;
  const kept = cookieHeader
    .split(";")
    .map((part) => part.trim())
    .filter((part) => sessionCookieNames.has(part.slice(0, part.indexOf("="))));
  return kept.length > 0 ? kept.join("; ") : null;
}

export async function getOwner(request: Request): Promise<OwnerResult> {
  const cookie = sessionCookieHeader(request.headers.get("Cookie"));
  if (!cookie) return { status: "anonymous" };

  const response = await fetch(new URL("/api/v1/auth/me", internalApiUrl), {
    headers: { Cookie: cookie, Accept: "application/json" },
    redirect: "error",
    signal: AbortSignal.timeout(5000),
  });
  if (response.status === 401) return { status: "anonymous" };
  if (response.status === 503) return { status: "not_configured" };
  if (!response.ok) {
    throw new Error(`auth/me returned ${response.status}`);
  }
  return { status: "authenticated", owner: (await response.json()) as Owner };
}

export const privateHeaders = {
  "Cache-Control": "no-store, private",
  "X-Robots-Tag": "noindex, nofollow",
};
