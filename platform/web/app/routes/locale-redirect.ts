import { redirect } from "react-router";
import { defaultLocale } from "~/i18n";

// Temporary redirect: the default locale may later be negotiated (web-v1.md §9).
export function loader() {
  return redirect(`/${defaultLocale}`, 302);
}
