export const locales = ["es", "en"] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = "es";

export function isLocale(value: string | undefined): value is Locale {
  return locales.includes(value as Locale);
}

// UI catalog only. Editorial content is translated manually in the admin panel (web-v1.md §9).
const messages = {
  es: {
    "site.tagline":
      "Construyo sistemas reales, documento cómo funcionan y publico los resultados.",
    "site.underConstruction":
      "Sitio en construcción. Todavía no hay contenido publicado.",
    "site.switchLocale": "English",
    "error.notFound": "No se encontró la página solicitada.",
    "error.generic": "Ocurrió un error inesperado.",
    "doc.youtube.play": "Reproducir el vídeo de YouTube",
    "doc.youtube.notice": "Al reproducirlo se conecta con YouTube (youtube-nocookie.com).",
    "doc.youtube.title": "Vídeo de YouTube",
    "doc.media.pending": "Archivo pendiente: la carga de archivos todavía no está disponible.",
  },
  en: {
    "site.tagline":
      "I build real systems, document how they work and publish the results.",
    "site.underConstruction":
      "Site under construction. No content has been published yet.",
    "site.switchLocale": "Español",
    "error.notFound": "The requested page could not be found.",
    "error.generic": "An unexpected error occurred.",
    "doc.youtube.play": "Play the YouTube video",
    "doc.youtube.notice": "Playing it connects to YouTube (youtube-nocookie.com).",
    "doc.youtube.title": "YouTube video",
    "doc.media.pending": "File pending: uploads are not available yet.",
  },
} satisfies Record<Locale, Record<string, string>>;

export type MessageKey = keyof (typeof messages)["es"];

export function t(locale: Locale, key: MessageKey): string {
  return messages[locale][key];
}
