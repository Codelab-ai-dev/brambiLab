import {
  type RouteConfig,
  index,
  route,
} from "@react-router/dev/routes";
import { sections } from "./site/paths";
import { locales } from "./i18n";

// Public site: the localized paths of web-v1.md §9, one route tree per locale (the same modules
// with locale-specific ids). An unknown locale or segment has no route: a real 404.
const site = locales.map((l) => {
  const s = sections[l];
  const id = (name: string) => ({ id: `${l}-${name}` });
  return route(l, "routes/locale-layout.tsx", id("layout"), [
    index("routes/home.tsx", id("home")),
    route(s.projects, "routes/site/projects.tsx", id("projects")),
    route(`${s.projects}/:slug`, "routes/site/project.tsx", id("project")),
    route(`${s.projects}/:projectSlug/${s.log}/:slug`, "routes/site/log.tsx", id("log")),
    route(s.articles, "routes/site/articles.tsx", id("articles")),
    route(`${s.articles}/:slug`, "routes/site/article.tsx", id("article")),
    route(s.search, "routes/site/search.tsx", id("search")),
    route(s.about, "routes/site/about.tsx", id("about")),
    route(s.contact, "routes/site/contact.tsx", id("contact")),
    route("*", "routes/site/not-found.tsx", id("not-found")),
  ]);
});

export default [
  index("routes/locale-redirect.ts"),
  route("healthz", "routes/healthz.ts"),
  route("robots.txt", "routes/robots.ts"),
  route("sitemap.xml", "routes/sitemap.ts"),
  route("sitemaps/:page", "routes/sitemap-page.ts"),
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
    route("medios", "routes/admin/media.tsx"),
    route("medios/:id", "routes/admin/media-detail.tsx"),
  ]),
  ...site,
] satisfies RouteConfig;
