import {
  type RouteConfig,
  index,
  route,
} from "@react-router/dev/routes";

export default [
  index("routes/locale-redirect.ts"),
  route("healthz", "routes/healthz.ts"),
  // Private admin (web-v1.md §9): Spanish UI, not under a locale prefix.
  route("admin/login", "routes/admin/login.tsx"),
  route("admin", "routes/admin/layout.tsx", [
    index("routes/admin/home.tsx"),
    route("contenidos", "routes/admin/contents.tsx"),
    route("contenidos/nuevo", "routes/admin/content-new.tsx"),
    route("contenidos/:id", "routes/admin/content.tsx"),
    route("contenidos/:id/:locale", "routes/admin/editor.tsx"),
    route("contenidos/:id/:locale/historial", "routes/admin/history.tsx"),
    route("contenidos/:id/:locale/v/:version", "routes/admin/preview.tsx"),
    route("taxonomia", "routes/admin/taxonomy.tsx"),
  ]),
  route(":lang", "routes/locale-layout.tsx", [index("routes/home.tsx")]),
] satisfies RouteConfig;
