import { describe, expect, it } from "vitest";
import { sessionCookieHeader } from "./api.server";

describe("sessionCookieHeader", () => {
  it("forwards only the session cookie", () => {
    expect(sessionCookieHeader("_ga=GA1.1; bl_session=abc; theme=dark")).toBe("bl_session=abc");
    expect(sessionCookieHeader("__Host-bl_session=xyz; other=1")).toBe("__Host-bl_session=xyz");
  });

  it("drops everything when there is no session cookie", () => {
    expect(sessionCookieHeader(null)).toBeNull();
    expect(sessionCookieHeader("")).toBeNull();
    expect(sessionCookieHeader("bl_oauth_state=s; bl_session_x=1; xbl_session=2")).toBeNull();
  });
});
