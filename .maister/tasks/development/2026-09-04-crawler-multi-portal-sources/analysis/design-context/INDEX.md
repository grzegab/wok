# Design Context — indeks

**Zadanie:** `2026-09-04-crawler-multi-portal-sources`
**Data:** 2026-09-04
**Zakres UI:** jedna kolumna `Source` w liście zescrapowanych ofert. Bez filtrowania, bez CRUD-u źródeł, bez podglądu HTML-a.

Źródło mockupów: `analysis/design-context/ascii/ui-mockups.md`

## Ekrany

| ID | Typ | Źródło | Opis |
|----|-----|--------|------|
| `screen:scraped-list` | screen | `ascii/ui-mockups.md#after` | Lista zescrapowanych ofert (`internal/templates/scraped/list.gohtml`) ze stanem docelowym: 5 kolumn, `Source` na pozycji drugiej |
| `screen:scraped-list-before` | screen | `ascii/ui-mockups.md#before` | Stan wyjściowy tej samej listy: 4 kolumny, punkt odniesienia dla diffa |
| `screen:scraped-list-empty` | screen | `ascii/ui-mockups.md#pusty-stan` | Pusty stan listy — `colspan="4"` → `colspan="5"` w `scraped/list.gohtml:29` |
| `screen:category-nav` | screen | `ascii/ui-mockups.md#categorynav` | Nagłówek kategorii z przełącznikiem Scraped/Stats. **Werdykt: bez zmian** |

## Komponenty

| ID | Typ | Źródło | Opis |
|----|-----|--------|------|
| `component:source-column-header` | component | `ascii/ui-mockups.md#after` | Nowy `<th>Source</th>` po `<th>Title</th>` (`scraped/list.gohtml`, po linii 13) |
| `component:source-cell` | component | `ascii/ui-mockups.md#after` | `<td class="text-nowrap">` z `<span class="badge text-bg-light border">{{ .Source }}</span>`; wzorzec badge'a z `offers/list.gohtml:40,44` |
| `component:possibility-row` | component | `ascii/ui-mockups.md#wiersz-normalny` | View model `possibilityRow` w `internal/handlers/scraped_handler.go:28-33` — dochodzi pole `Source string` |
| `component:row-no-published-date` | component | `ascii/ui-mockups.md#wiersz-bez-daty` | Wariant wiersza z `posted_at IS NULL`; `formatDate` (`helper.go:36`) już zwraca `-` — zmiana niewymagana |
| `component:row-duplicate-across-sources` | component | `ascii/ui-mockups.md#wiersz-duplikat` | Ta sama oferta w dwóch wierszach z różnych portali; przesądza o umiejscowieniu kolumny obok `Title` |
| `component:row-unparsed-placeholder` | component | `ascii/ui-mockups.md#wiersz-placeholder` | Wiersz po nieudanym parsowaniu — `text-muted fst-italic` zamiast `fw-medium` w komórce tytułu; wariant A/B do rozstrzygnięcia |
| `component:source-null-fallback` | component | `ascii/ui-mockups.md#reuse` | Render `source_id IS NULL` (wiersze sprzed backfillu) jako `-`, bez badge'a |
| `component:empty-state-row` | component | `ascii/ui-mockups.md#pusty-stan` | Wiersz „No scraped offers yet." — `colspan` do podniesienia |

## Bez zmian (potwierdzone)

| ID | Typ | Plik | Uzasadnienie |
|----|-----|------|--------------|
| `component:categorynav` | component | `internal/templates/partials/categorynav.gohtml` | Operuje tylko na `.Category.Name`/`.Category.Slug` i `.Page`; brak referencji do portali — patrz `ascii/ui-mockups.md#categorynav` |
| `component:header-nav` | component | `internal/templates/partials/header.gohtml` | Nawigacja po kategoriach, niezależna od źródła |
| `component:page-head` | component | `internal/templates/partials/head.gohtml` | Bootstrap z `/assets/bootstrap.min.css`; nowa kolumna nie wnosi CSS ani JS |
| `component:page-footer` | component | `internal/templates/partials/footer.gohtml` | — |
| `screen:stats` | screen | `internal/templates/stats/stats.gohtml` | Statystyki agregują `tech` per kategoria, nie per źródło; rozbicie po portalu poza zakresem |

## Decyzje otwarte dla planisty

| ID | Kwestia | Sekcja |
|----|---------|--------|
| `decision:unparsed-flag` | `{{ if eq .JobTitle "(unparsed)" }}` w szablonie (A) vs `possibilityRow.Unparsed bool` ze wspólnej stałej (B, rekomendowane, +1 pole ponad zatwierdzony zakres) | `ascii/ui-mockups.md#wiersz-placeholder` |
| `decision:source-name-lookup` | `LEFT JOIN sources` w `ListPossibilitiesByCategory` vs `map[int64]string` w handlerze; render identyczny w obu wariantach | `ascii/ui-mockups.md#reuse` |
