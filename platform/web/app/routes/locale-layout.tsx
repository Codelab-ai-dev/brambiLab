import { data, Outlet } from "react-router";
import type { Route } from "./+types/locale-layout";
import { isLocale } from "~/i18n";

export function loader({ params }: Route.LoaderArgs) {
  if (!isLocale(params.lang)) {
    throw data(null, { status: 404 });
  }
  return { locale: params.lang };
}

export default function LocaleLayout() {
  return <Outlet />;
}
