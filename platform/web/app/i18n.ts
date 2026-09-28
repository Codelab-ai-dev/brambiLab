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
    "site.lab": "Laboratorio personal de Gustavo González para software, IA, microcontroladores, IoT y robótica.",
    "site.status": "En construcción · todavía no hay contenido publicado",
    "site.areas": "Áreas",
    "site.area.software": "software",
    "site.area.ai": "IA",
    "site.area.mcu": "microcontroladores",
    "site.area.iot": "IoT",
    "site.area.robotics": "robótica",
    "site.publish.title": "Qué se publicará aquí",
    "site.publish.projects.title": "Proyectos",
    "site.publish.projects.text": "Fichas técnicas con objetivo, estado, tecnologías, enlaces y resultados de cada desarrollo.",
    "site.publish.log.title": "Bitácora",
    "site.publish.log.text": "Entradas de avance de cada proyecto desde sus primeros experimentos, con lo que funcionó y lo que no.",
    "site.publish.articles.title": "Artículos",
    "site.publish.articles.text": "Textos independientes sobre técnicas, herramientas y decisiones de ingeniería.",
    "site.soon": "próximamente",
    "site.method.title": "Cómo se documenta",
    "site.method.hypothesis": "Hipótesis y resultados, separados.",
    "site.method.failures": "Los fallos también se documentan.",
    "site.method.evidence": "Evidencia: fotos, mediciones y logs, con sus limitaciones.",
    "site.footer.licenses": "Textos y medios CC BY 4.0 · código Apache-2.0 · diseños CERN-OHL-W-2.0",
    "site.footer.source": "Código fuente",
    "site.switchLocaleLabel": "Read this page in English",
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
    "site.lab": "Gustavo González's personal lab for software, AI, microcontrollers, IoT and robotics.",
    "site.status": "Under construction · nothing published yet",
    "site.areas": "Areas",
    "site.area.software": "software",
    "site.area.ai": "AI",
    "site.area.mcu": "microcontrollers",
    "site.area.iot": "IoT",
    "site.area.robotics": "robotics",
    "site.publish.title": "What will be published here",
    "site.publish.projects.title": "Projects",
    "site.publish.projects.text": "Technical sheets with each build's goal, status, technologies, links and results.",
    "site.publish.log.title": "Build log",
    "site.publish.log.text": "Progress entries for every project from its first experiments, including what worked and what did not.",
    "site.publish.articles.title": "Articles",
    "site.publish.articles.text": "Standalone writing on techniques, tools and engineering decisions.",
    "site.soon": "coming soon",
    "site.method.title": "How it is documented",
    "site.method.hypothesis": "Hypotheses and results, kept apart.",
    "site.method.failures": "Failures are documented too.",
    "site.method.evidence": "Evidence: photos, measurements and logs, with their limitations.",
    "site.footer.licenses": "Text and media CC BY 4.0 · code Apache-2.0 · designs CERN-OHL-W-2.0",
    "site.footer.source": "Source code",
    "site.switchLocaleLabel": "Leer esta página en español",
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
