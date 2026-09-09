# Rozstrzygnięcia zakresu — Faza 2

Data: 2026-09-04
Źródło: bramka decyzyjna po `analysis/gap-analysis.md` (8 decyzji krytycznych, 9 ważnych).

## Decyzje krytyczne

| # | Decyzja | Wybór |
|---|---|---|
| ddl-delivery | Dostarczenie DDL na żywą bazę | **Nazwany plik migracji** `001_multi_portal_sources.sql`, aplikowany przez `deploy.sh`, bez tabeli `schema_migrations` |
| seed-placement | Umiejscowienie seedu 23 wierszy | **Osobny `seed.sql`**, włączany zarówno przez świeżą instalację, jak i przez deploy |
| category-source-of-truth | Los `category.All` | **Zostaje w Go**, znika wyłącznie pole `ScrapeURL`; do bazy idą tylko źródła i URL-e |
| category-key-mismatch | Klucz kategorii | **Crawler mapuje slug → Name** przez `category.BySlug(slug).Name` tuż przed `CreatePossibilityParams`; `possibilities.category` nadal trzyma `Name` |
| url-canonicalization | Postać zapisywanego URL-a | **Wymuszenie URL-a absolutnego**, bez obcinania query stringu i fragmentu |
| possibility-html-orphan | Retencja surowego HTML | **Write-only bez retencji** — brak polityki kasowania, brak wyłączania z backupu |
| sources-crud-orphan | Zarządzanie źródłami | **`psql` jako jedyny interfejs administracyjny** |
| failed-parse-policy | Nieudane parsowanie oferty | **Zapis wiersza z surowym HTML-em i rozpoznawalnym placeholderem** w `job_title`/`company_name` |

### Konsekwencje wymagające obsługi w specyfikacji

**`url-canonicalization` — obowiązkowy krok przed napisaniem DDL.**
Trzeba sprawdzić na produkcji, w jakiej postaci są istniejące wiersze:
```sql
SELECT url FROM possibilities LIMIT 5;
```
Jeśli są relatywne, do migracji dochodzi jednorazowy `UPDATE` normalizujący je do postaci absolutnej. Bez tego kroku normalizacja utworzy duplikaty wszystkich istniejących ofert justjoin.it.

**`possibility-html-orphan` — świadomie przyjęty dług.**
`scripts/deploy.sh:24-29` robi pełny `pg_dump` przy każdym wdrożeniu. Bez retencji i bez wyłączenia tabeli z dumpu każdy kolejny deploy będzie wolniejszy, a rozmiar backupu będzie rósł nieograniczenie. Decyzja użytkownika: akceptujemy, kasowanie ręczne gdy zaboli. **Do odnotowania w specyfikacji jako znane ograniczenie, nie do naprawiania w tym zadaniu.**

## Decyzje ważne — zatwierdzone

- **Kolumna `Source` w liście ofert** — pole w `possibilityRow`, mapa `source_id` → nazwa w handlerze, jeden `<th>`/`<td>` w `internal/templates/scraped/list.gohtml`. Bez filtrowania po źródle.
- **Throttling jako jeden blok `const`** — 4 stałe z `fetcher.go:16-17` i `transformer.go:20-21` zebrane w jedno miejsce. Bez zmiennych środowiskowych `CRAWLER_*`.
- **Bulldogjob `skills,C` jako 22. wiersz seedu** — `https://bulldogjob.pl/companies/jobs/s/skills,C`, 6 ofert nieobjętych przez `skills,C++`. Łączna liczba wierszy `scrape_targets`: **23**.
- **Flaga `slices.Reverse` per portal** — `fetcher.go:110` przestaje odwracać listę bezwarunkowo. Włączona dla justjoin.it i NoFluffJobs (sortowanie potwierdzone), wyłączona dla Bulldogjob i solid.jobs (brak parametru sortowania).
- **Limit 3 stron na target dla paginacji Bulldogjob** — ~150 ofert na target na przebieg, z logiem przy osiągnięciu limitu.
- **`disable-dev-shm-usage` + `mem_limit: 4g`** — `chromedp.Flag("disable-dev-shm-usage", true)` omija domyślne 64 MB `/dev/shm`, plus limit pamięci kontenera crawlera. `init: true` jest już ustawione w `docker-compose.yaml:51`.

## Decyzje ważne — ODRZUCONE i ich skutki

**1. Test integracyjny Postgresa ze self-skip — odrzucony.**
`ON CONFLICT (url) DO UPDATE` pozostaje bez pokrycia testami, mimo że jest objęty jawnym zakazem regresji. Testy ograniczają się do jednostkowych (parsery per portal na fixture'ach HTML, helpery). **Ryzyko:** żaden test nie wykryje, jeśli zmiana w warstwie zapisu po cichu zepsuje deduplikację. Do odnotowania w specyfikacji jako świadomie przyjęte ryzyko.

**2. „Bez limitu ofert na pierwszy backfill" — odrzucone, czyli limit ma być.**
Użytkownik nie akceptuje wielodniowego backfillu bez ogranicznika. Potrzebny **limit nowych ofert na przebieg per portal**, niezależny od limitu 3 stron listingu (ten ogranicza odkrywanie URL-i, nie liczbę scrapowanych ofert).
**Do rozstrzygnięcia w Fazie 5:** konkretna wartość limitu. Punkt odniesienia: przy 2-3 min na ofertę i przebiegu co 4-5 h jeden portal zdąży przerobić ~100-150 ofert, zanim ruszy kolejny przebieg.

**3. Transakcja `txBeginner` + `pgPool` — odrzucona.**
Zapis `possibilities` i `possibility_html` idzie dwoma osobnymi `INSERT`-ami: najpierw oferta z `RETURNING id`, potem HTML. **Ryzyko:** jeśli drugi `INSERT` padnie (timeout przy setkach KB, restart kontenera między zapisami), zostaje wiersz oferty bez zapisanego HTML-a — czyli dokładnie ta sytuacja, przed którą miał chronić punkt (c) zadania. Nie ma automatycznego wykrywania takich sierot.
**Do rozstrzygnięcia w Fazie 5:** czy timeout w `internal/justjoin/persist.go:34` (dziś `3 * time.Second`) podnieść, skoro zapis setek KB idzie teraz bez transakcji i z 4 równoległych pipeline'ów.

## Stan po Fazie 2

- Ryzyko: **high**; wpływ architektoniczny: **high**; typ zmiany: `modificative`, kompatybilność `strict`.
- `task_characteristics`: `modifies_existing_code`, `creates_new_entities`, `involves_data_operations`, `ui_heavy` — wszystkie **true**; `has_reproducible_defect` — false.
- Faza 4 (mockupy UI) — **aktywna**, zatwierdzona przez użytkownika.
- E2E i dokumentacja użytkownika — włączone domyślnie, do potwierdzenia w Fazie 10.
