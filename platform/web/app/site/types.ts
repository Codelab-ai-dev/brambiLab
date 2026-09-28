// Shapes of the public API (platform/contracts/openapi.yaml, WEB-006). Only published, visible data.
import type { Doc } from "~/content/schema";
import type { Locale } from "~/i18n";
import type { ContentKind } from "./paths";

export type Term = { slug: string; label: string; count?: number };
export type Cover = { asset_id: string; width: number | null; height: number | null; alt: string };
export type ProjectStatus = "idea" | "in_development" | "paused" | "completed";

export type Card = {
  kind: ContentKind;
  slug: string;
  title: string;
  summary: string;
  published_at: string;
  first_published_at: string;
  category: Term | null;
  tags: Term[];
  project: { slug: string; title: string } | null;
  project_fields: { status?: ProjectStatus; technologies?: string[] } | null;
  cover: Cover | null;
  snippet?: { text: string; hit?: boolean }[];
};

export type CardPage = { items: Card[]; page: number; page_size: number; total: number };
export type SearchPage = CardPage & { query: string; has_terms: boolean };
export type Taxonomy = { categories: Term[]; tags: Term[] };

export type SiteLink = { kind: "github" | "linkedin" | "website" | "other"; label: string; url: string };
export type PublicSite = { intro: string; bio: string; contact_email: string; links: SiteLink[] };
export type Home = { site: PublicSite; featured: Card[]; latest_projects: Card[]; latest_logs: Card[]; latest_articles: Card[] };

export type PublicAsset = { kind: "image" | "video" | "resource"; mime: string; width: number | null; height: number | null; bytes: number; downloadable: boolean; name: string };
export type Alternate = { locale: Locale; kind: ContentKind; slug: string; project_slug: string | null };

export type PublicContent = {
  kind: ContentKind;
  locale: Locale;
  slug: string;
  title: string;
  summary: string;
  body: Doc;
  seo: { title?: string; description?: string };
  project_fields?: {
    objective?: string;
    status?: ProjectStatus;
    technologies?: string[];
    links?: { label: string; url: string }[];
    results?: string;
  };
  cover: Cover | null;
  published_at: string;
  first_published_at: string;
  project?: { slug: string; title: string };
  category: Term | null;
  tags: Term[];
  alternates: Alternate[];
  assets: Record<string, PublicAsset>;
};

export type SitemapPage = {
  items: { kind: ContentKind; locale: Locale; slug: string; project_slug: string | null; lastmod: string; alternates: { locale: Locale; slug: string; project_slug: string | null }[] }[];
  page: number;
  page_size: number;
  total: number;
};

/** Where the language switcher goes: the same page in the other locale, or its index when missing. */
export type Switcher = { href: string; available: boolean };
