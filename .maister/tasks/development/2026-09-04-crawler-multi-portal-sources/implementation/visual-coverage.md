# Macierz pokrycia wizualnego

**Zadanie:** `2026-09-04-crawler-multi-portal-sources`
**Źródło:** `analysis/design-context/INDEX.md`
**Mockupy:** `analysis/design-context/ascii/ui-mockups.md`
**Plan:** `implementation/implementation-plan.md`

Fidelity mockupów jest ASCII i orientacyjne. **Wiążące są: liczba i kolejność kolumn, klasy Bootstrapa i wartość `colspan`** (`spec.md` §10). Proporcje szerokości nie są wiążące.

## Ekrany

| ID ekranu | Pokryte przez grupę | Zakres odpowiedzialności | Status |
|---|---|---|---|
| `screen:scraped-list` | **Grupa 12** (UI — kolumna `Source`) | 5 kolumn w kolejności `Title, Source, Published, Technologies, URL`; kontener tabeli bez zmian | ✅ |
| `screen:scraped-list-before` | **Grupa 12** | punkt odniesienia dla `git diff` — potwierdzenie, że zmienia się wyłącznie to, co odróżnia `#before` od `#after` | ✅ |
| `screen:scraped-list-empty` | **Grupa 12** (krok 12.7) | `colspan="4"` → `colspan="5"` w `internal/templates/scraped/list.gohtml:29` | ✅ |
| `screen:category-nav` | **Grupa 12** (weryfikacja braku zmian) | werdykt „bez zmian" — pusty `git diff` na `internal/templates/partials/categorynav.gohtml`; egzekwowane przez kryterium akceptacji 20 | ✅ |

## Komponenty

| ID komponentu | Pokryte przez grupę | Zakres odpowiedzialności | Status |
|---|---|---|---|
| `component:source-column-header` | **Grupa 12** (krok 12.4) | nowy `<th>Source</th>` bezpośrednio po `<th>Title</th>` | ✅ |
| `component:source-cell` | **Grupa 12** (krok 12.5) + **Grupa 4** (dane) | `<td class="text-nowrap">` z badge `text-bg-light border`; wartość pochodzi z `LEFT JOIN sources` w `ListPossibilitiesByCategory` (rozstrzygnięcie `decision:source-name-lookup`) | ✅ |
| `component:possibility-row` | **Grupa 12** (kroki 12.2, 12.3) | `possibilityRow` dostaje `Source string` i `Unparsed bool` | ✅ |
| `component:row-no-published-date` | **Grupa 12** (test w kroku 12.1) | zmiana niewymagana — `formatDate` (`helper.go:36-41`) już zwraca `-`; pokrycie polega na potwierdzeniu testem, nie na kodzie | ✅ |
| `component:row-duplicate-across-sources` | **Grupa 12** (kroki 12.4, 12.5) | rozróżnialność dwóch wierszy tej samej oferty z różnych portali; przesądza o umieszczeniu `Source` obok `Title` | ✅ |
| `component:row-unparsed-placeholder` | **Grupa 12** (krok 12.6) + **Grupa 5** (stała) + **Grupa 6** (polityka zapisu) | wariant **B**: `text-muted fst-italic` zamiast `fw-medium`, sterowane `possibilityRow.Unparsed` ze wspólnej stałej `internal/offer.Unparsed` (rozstrzygnięcie `decision:unparsed-flag`) | ✅ |
| `component:source-null-fallback` | **Grupa 12** (krok 12.5) + **Grupa 3** (backfill) | gołe `-` bez `<span class="badge">`; stan `source_id IS NULL` powstaje dla wierszy, których nie obejmie `data/002_backfill_source_id.sql` | ✅ |
| `component:empty-state-row` | **Grupa 12** (krok 12.7) | `colspan="5"` w wierszu „No scraped offers yet." | ✅ |

## Elementy oznaczone „bez zmian" — pokrycie przez weryfikację

Te pozycje `INDEX.md` nie wymagają kodu, ale wymagają dowodu, że nic ich nie ruszyło. Dowodem jest kryterium akceptacji 20 (`git diff internal/templates/` nie dodaje CSS, JS ani partiala) sprawdzane w kroku 12.8.

| ID | Pokryte przez grupę | Status |
|---|---|---|
| `component:categorynav` | **Grupa 12** (krok 12.8, weryfikacja diffa) | ✅ |
| `component:header-nav` | **Grupa 12** (krok 12.8) | ✅ |
| `component:page-head` | **Grupa 12** (krok 12.8) | ✅ |
| `component:page-footer` | **Grupa 12** (krok 12.8) | ✅ |
| `screen:stats` | **Grupa 12** (krok 12.8) + weryfikacja `200` dla `/st4ts/{category}` w **Grupie 13** (P4) | ✅ |

## Decyzje otwarte dla planisty — rozstrzygnięcia

`INDEX.md` przekazał planiście dwie decyzje. Obie są rozstrzygnięte w specyfikacji i przypisane do grup:

| ID decyzji | Rozstrzygnięcie | Grupa realizująca |
|---|---|---|
| `decision:unparsed-flag` | **wariant B** — `possibilityRow.Unparsed bool` zasilane ze wspólnej stałej `internal/offer.Unparsed`, zamiast literału `"(unparsed)"` w szablonie (`spec.md` §10.2, `technical-clarifications.md` p. 5) | Grupa 5 (stała), Grupa 12 (pole i render) |
| `decision:source-name-lookup` | **`LEFT JOIN sources`** w `ListPossibilitiesByCategory`, nie `map[int64]string` w handlerze — mapa byłaby cache'em nieodświeżalnym bez restartu, a źródła zmienia się przez `psql` (`spec.md` §10.4) | Grupa 4 (zapytanie), Grupa 12 (mapowanie) |

## Elementy niepokryte

**Brak.** Wszystkie 4 ekrany i 8 komponentów z sekcji zmian `analysis/design-context/INDEX.md`, a także wszystkie 5 pozycji z sekcji „Bez zmian (potwierdzone)", mają przypisaną co najmniej jedną grupę zadań.

Poza zakresem tego zadania — i **świadomie nieobecne** w `INDEX.md`, więc nieobjęte tą macierzą: filtrowanie po źródle, statystyki per źródło, CRUD źródeł w UI, podgląd surowego HTML-a, trasa `/scr4p3d/{category}/{id}/html`, paginacja listy ofert (`spec.md` §3.2).
