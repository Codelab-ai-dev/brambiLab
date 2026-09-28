import { getLoginAvailability, getOwner, type LoginAvailability } from "./api.server";

export type LoginState =
  | { kind: "redirect"; to: string }
  | { kind: "page"; availability: LoginAvailability; returnTo: string };

/**
 * Decides what /admin/login shows. Session state (a valid owner goes straight to the panel) and
 * login availability (is the GitHub button useful?) are separate questions; Go answers both.
 */
export async function loginState(request: Request): Promise<LoginState> {
  const url = new URL(request.url);
  // Go validates return_to again; this only keeps the link tidy.
  const rawReturnTo = url.searchParams.get("return_to") ?? "/admin";
  const returnTo = rawReturnTo === "/admin" || rawReturnTo.startsWith("/admin/") ? rawReturnTo : "/admin";

  try {
    const owner = await getOwner(request);
    if (owner.status === "authenticated") return { kind: "redirect", to: returnTo };
  } catch {
    // The session check failed (API down); the availability check below reports it.
  }
  return { kind: "page", availability: await getLoginAvailability(), returnTo };
}
