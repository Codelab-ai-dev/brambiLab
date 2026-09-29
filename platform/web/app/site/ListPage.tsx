import { t, type MessageKey } from "~/i18n";
import type { ListData } from "./list.server";
import { ContentList, Empty, FilterForm, Frame, PageHeader, Pager } from "./ui";

export function ListPage({ data }: { data: ListData }) {
  const { locale, kind, page, taxonomy, selected, path } = data;
  const section = kind === "project" ? "projects" : "articles";
  const query = new URLSearchParams();
  if (selected.category) query.set("category", selected.category);
  if (selected.tag) query.set("tag", selected.tag);
  const filtered = query.size > 0;
  // The count is the API total for this language (and these filters), never the page length.
  const count = `${page.total} ${t(locale, `list.${section}.count${page.total === 1 ? "One" : ""}` as MessageKey)}`;
  return (
    <main>
      <PageHeader eyebrow={count} title={t(locale, `list.${section}.title` as MessageKey)} lead={t(locale, `list.${section}.lead` as MessageKey)}>
        {(taxonomy.categories.length > 0 || taxonomy.tags.length > 0) && (
          <div className="mt-12 border-t border-border pt-8">
            <FilterForm locale={locale} action={path} categories={taxonomy.categories} tags={taxonomy.tags} selected={selected} />
          </div>
        )}
      </PageHeader>
      <Frame className="py-12 sm:py-16">
        {page.items.length > 0 ? (
          <ContentList locale={locale} cards={page.items} headingLevel={2} />
        ) : (
          <Empty>{t(locale, filtered ? "list.emptyFiltered" : "list.empty")}</Empty>
        )}
        <Pager locale={locale} page={page.page} pageSize={page.page_size} total={page.total} path={path} query={query} />
      </Frame>
    </main>
  );
}
