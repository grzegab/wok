# Wymagania — Faza 5B

Data: 2026-09-04

## Opis wyjściowy

Dodać do crawlera nowe portale z ofertami pracy: NoFluffJobs, Bulldogjob, solid.jobs. Najpierw ustawić źródło w bazie danych, potem URL, pod który strzelamy, na koniec zapisywać HTML oraz — jeśli jest możliwość — słowa kluczowe i datę pojawienia się oferty. Deduplikacja semantyczna poza zakresem (użytkownik zrobi ją ręcznie z pomocą LLM); tutaj tylko pobieramy dane. Throttling: oferta co 2-3 min, lista ofert co 4-5h z randomizacją.

## Wymagania funkcjonalne

### F1. Rejestr źródeł w bazie
- Tabela `sources` — portal (slug, nazwa).
- Tabela `scrape_targets` — `source_id` × kategoria × URL listingu × `enabled`. Model dopuszcza **wiele URL-i na parę portal×kategoria**.
- `possibilities.source_id BIGINT REFERENCES sources`, indeksowany, **nullable** (wiersze sprzed backfillu).
- Seed: **23 wiersze** `scrape_targets` — patrz `analysis/approved-urls.md` (justjoin.it 5, nofluffjobs.com 7, bulldogjob.pl 6, solid.jobs 5).
- Zarządzanie zawartością wyłącznie przez `psql`. Bez CRUD-u w UI.

### F2. Odkrywanie ofert per portal
- Interfejs `linkCollector` w `fetcher.go`; jedna implementacja na portal.
- justjoin.it: infinite scroll (istniejące zachowanie). Bulldogjob: paginacja, **limit 3 stron na target na przebieg**, z logiem przy osiągnięciu limitu. NoFluffJobs i solid.jobs: mechanika do ustalenia przy implementacji.
- `slices.Reverse` (`fetcher.go:110`) staje się **flagą per portal**: włączona dla justjoin.it i NoFluffJobs (sortowanie newest-first potwierdzone), wyłączona dla Bulldogjob i solid.jobs (brak parametru sortowania).
- URL oferty **wymuszany do postaci absolutnej** przed zapisem. Bez obcinania query stringu i fragmentu.

### F3. Parsowanie ofert per portal
- Rozdzielenie na pobranie surowego HTML (chromedp) i parsowanie w czystej funkcji Go.
- Interfejs `offerParser` w `transformer.go`; jedna implementacja na portal, łącznie z przepisanym justjoin.it.
- Słowa kluczowe trafiają do **istniejącej kolumny `tech`** w formacie rozdzielanym `", "`. Bez nowej kolumny `keywords`.
- Data publikacji do istniejącej kolumny `posted_at` (nullable), parsowana per portal w Go.

### F4. Zapis surowego HTML
- Tabela `possibility_html(possibility_id PK/FK, html TEXT, fetched_at)`, relacja 1:1.
- Zapis **dwoma osobnymi `INSERT`-ami** (bez transakcji): oferta z `RETURNING id`, potem HTML.
- Timeout zapisu: **60 s** (dziś 3 s).
- Tabela write-only — nic w aplikacji jej nie czyta. Bez polityki retencji.

### F5. Nieudane parsowanie
- Oferta, której nie udało się sparsować, **i tak trafia do bazy** z surowym HTML-em i rozpoznawalnym placeholderem w `job_title`/`company_name` (oba `NOT NULL`).
- Stała placeholdera w jednym miejscu, współdzielona przez crawlera i handler.

### F6. Throttling
- Pauza między ofertami: **2-3 min** (dziś 5-12 s, `transformer.go:20-21`).
- Interwał listingu: **4-5 h** z randomizacją (dziś 2 h + 1 h jitteru, `fetcher.go:16-17`).
- Mechanizm randomizacji (`rand.N`) i anulowalne czekanie (`pause()`, `transformer.go:135`) bez zmian.
- Cztery stałe zebrane w **jeden blok `const`**. Bez zmiennych środowiskowych.
- **Limit 100 ofert na przebieg per portal.**

### F7. Równoległość per portal
- Cztery niezależne pipeline'y, każdy z własną instancją Chromium i własnym throttlingiem.
- Graceful shutdown musi objąć N gałęzi bez podwójnego zamknięcia kanału.
- `chromedp.Flag("disable-dev-shm-usage", true)` + `mem_limit: 4g` dla kontenera crawlera.

### F8. Kolumna Source w UI
- `internal/templates/scraped/list.gohtml`: kolumna **druga**, między `Title` a `Published`.
- Pełna nazwa portalu w `<span class="badge text-bg-light border">` — wzorzec z `offers/list.gohtml:40,44`, zero nowego CSS.
- Nazwa pobierana przez `LEFT JOIN sources` w `ListPossibilitiesByCategory`.
- `source_id IS NULL` (wiersze sprzed backfillu) renderuje `-`, zgodnie z istniejącą konwencją `formatDate`/`formatText`.
- `colspan="4"` w pustym stanie (`list.gohtml:29`) → **`5`**.
- `possibilityRow` dostaje pola na nazwę źródła i `Unparsed bool`.
- Bez filtrowania po źródle. `categorynav.gohtml` bez zmian.

### F9. Dostarczenie zmian schematu
- Nazwany plik migracji `001_multi_portal_sources.sql`, aplikowany przez `scripts/deploy.sh`. Bez tabeli `schema_migrations`.
- Osobny `seed.sql` z 23 wierszami, włączany zarówno przez świeżą instalację, jak i przez deploy.
- Całe DDL oparte na `current_schema()`, **nigdy** na `public` (schemat `wok` na produkcji).
- `category.All` zostaje w Go; znika wyłącznie pole `ScrapeURL`.
- Crawler mapuje `category.BySlug(slug).Name` przed złożeniem `CreatePossibilityParams` — `possibilities.category` nadal trzyma `Name`.

## Wymagania niefunkcjonalne

- **Zakaz regresji dedupu po URL**: `UNIQUE (url)`, `ON CONFLICT (url) DO UPDATE`, `ExistingPossibilityUrls` — bez zmian w zachowaniu.
- Istniejące widoki `/scr4p3d/{category}` i `/st4ts/{category}` muszą działać nieprzerwanie, również dla danych sprzed migracji.
- Testy: fixture'y HTML per portal + testy tabelaryczne parserów, stdlib `testing`.

## Poza zakresem

- Deduplikacja semantyczna między portalami.
- Normalizacja nazw technologii w kolumnie `tech`.
- Filtrowanie i statystyki per źródło.
- CRUD źródeł w UI, podgląd surowego HTML w UI.
- Retencja i backup `possibility_html`.
- Test integracyjny przeciw realnemu Postgresowi.
- Zmienne środowiskowe `CRAWLER_*` dla throttlingu.
- Framework migracji z ewidencją zastosowanych zmian.

## Kroki wymagane PRZED implementacją DDL

Sprawdzić postać istniejących URL-i na produkcji:
```sql
SELECT url FROM possibilities LIMIT 5;
```
Jeśli są relatywne, do migracji dochodzi jednorazowy `UPDATE` normalizujący je do postaci absolutnej. Bez tego kroku wymuszenie absolutnych URL-i utworzy duplikaty wszystkich istniejących ofert justjoin.it.

## Możliwości ponownego użycia

| Do wykorzystania | Gdzie |
|---|---|
| Idiom wąskiego interfejsu u konsumenta | `transformer.go:28`, `persist.go:11`, `router/routing.go:50-58` |
| Anulowalne czekanie `pause(ctx, d)` | `transformer.go:135` |
| Randomizacja `rand.N` | `fetcher.go:50`, `transformer.go` |
| Cykl życia przeglądarki `newBrowser`/`newTab` | `browser.go:20,47` |
| Badge `text-bg-light border` | `offers/list.gohtml:40,44` |
| Konwencja `-` dla pustych wartości | `helper.go:36` (`formatDate`, `formatText`) |
| Idiom testów tabelarycznych ze stubami | `transformer_test.go` |
| `current_schema()` w idempotentnym DDL | `scripts/deploy.sh:62` |

## Materiały wizualne

Mockupy ASCII w `analysis/design-context/ascii/ui-mockups.md`, indeks w `analysis/design-context/INDEX.md`. Brak zewnętrznych makiet.
