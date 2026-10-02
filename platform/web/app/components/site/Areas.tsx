// Areas of work (WEB-009 §3): three editorial rows describing the lab's focus. They are not
// projects: no status, no links to pages that do not exist, and the note says so.
import { t, type Locale, type MessageKey } from "~/i18n";
import { Eyebrow, Frame, SectionHeader } from "./editorial";
import { NodeNetwork, PointCloud, Trajectory } from "./textures";

const areas = [
  { key: "robotics", Visual: Trajectory },
  { key: "ai", Visual: PointCloud },
  { key: "connected", Visual: NodeNetwork },
] as const;

export const AREA_COUNT = areas.length;

export function Areas({ locale, index }: { locale: Locale; index: string }) {
  const k = (area: string, part: string) => `areas.${area}.${part}` as MessageKey;
  return (
    <section aria-labelledby="areas-title" className="border-t border-border">
      <Frame className="py-20 sm:py-28">
        <div data-reveal className="grid gap-8 lg:grid-cols-12">
          <div className="lg:col-span-8">
            <SectionHeader id="areas-title" index={index} label={t(locale, "hero.focus")} title={t(locale, "areas.title")} />
          </div>
          <p className="self-end text-lg text-text-muted lg:col-span-4">{t(locale, "areas.note")}</p>
        </div>
        <ol className="mt-16">
          {areas.map(({ key, Visual }, i) => (
            <li key={key} data-reveal className="grid gap-8 border-t border-border py-12 sm:grid-cols-6 lg:grid-cols-12 lg:gap-8">
              <div className="min-w-0 sm:col-span-6 lg:col-span-5">
                <Eyebrow index={String(i + 1).padStart(2, "0")}>{t(locale, k(key, "short"))}</Eyebrow>
                <h3 className="mt-5 text-[clamp(2rem,4vw,4rem)] leading-[0.98] font-bold tracking-[-0.03em] [overflow-wrap:break-word]">{t(locale, k(key, "title"))}</h3>
              </div>
              <div className="min-w-0 sm:col-span-3 lg:col-span-4">
                <p className="text-lg leading-relaxed text-text-muted">{t(locale, k(key, "text"))}</p>
                <p className="mt-6 font-mono text-xs text-text">
                  <span className="text-text-muted uppercase tracking-[0.12em]">{t(locale, "areas.fields")} / </span>
                  {t(locale, k(key, "fields"))}
                </p>
              </div>
              <div className="border border-border text-text-muted sm:col-span-3 lg:col-span-3">
                <Visual className="h-auto w-full" />
              </div>
            </li>
          ))}
        </ol>
      </Frame>
    </section>
  );
}
