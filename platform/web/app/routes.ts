import {
  type RouteConfig,
  index,
  route,
} from "@react-router/dev/routes";

export default [
  index("routes/locale-redirect.ts"),
  route("healthz", "routes/healthz.ts"),
  route(":lang", "routes/locale-layout.tsx", [index("routes/home.tsx")]),
] satisfies RouteConfig;
