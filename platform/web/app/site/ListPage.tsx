import { t, type MessageKey } from "~/i18n";
import type { ListData } from "./list.server";
import { CardGrid, Container, Empty, FilterForm, PageIntro, Pager } from "./ui";

export function ListPage({ data }: { data: ListData }) {
  const { locale, kind, page, taxonomy, selected, path } = data;
  const section = kind === "project" ? "projects" : "articles";
  const query = new URLSearchParams();
  if (selected.category) query.set("category", selected.category);
  if (selected.tag) query.set("tag", selected.tag);
  const filtered = query.size > 0;
  return (
    <main>
      <PageIntro eyebrow={`${page.total}`} title={t(locale, `list.${section}.title` as MessageKey)} lead={t(locale, `list.${section}.lead` as MessageKey)}>
        {(taxonomy.categories.length > 0 || taxonomy.tags.length > 0) && (
          <div className="mt-8">
            <FilterForm locale={locale} action={path} categories={taxonomy.categories} tags={taxonomy.tags} selected={selected} />
          </div>
        )}
      </PageIntro>
      <Container className="py-10">
        {page.items.length > 0 ? (
          <CardGrid locale={locale} cards={page.items} headingLevel={2} />
        ) : (
          <Empty>{t(locale, filtered ? "list.emptyFiltered" : "list.empty")}</Empty>
        )}
        <Pager locale={locale} page={page.page} pageSize={page.page_size} total={page.total} path={path} query={query} />
      </Container>
    </main>
  );
}
