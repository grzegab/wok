# Plan implementacji: crawler wieloportalowy (NoFluffJobs, Bulldogjob, solid.jobs)

**Zadanie:** `2026-09-04-crawler-multi-portal-sources`
**Data:** 2026-09-04
**Źródło prawdy:** `implementation/spec.md` (1021 wierszy). **Aneks A jest nadrzędny** wobec sekcji inline tam, gdzie się różnią.

## Przegląd

| Miara | Wartość |
|---|---|
| Grupy zadań | 14 |
| Kroki łącznie | 91 |
| Testy bazowe (grupy 5-12) | 41 |
| Testy dodatkowe (grupa 14) | max 10 |
| Testy łącznie | **41-51** |
| Grupa z ręczną weryfikacją produkcji **przed** DDL | **Grupa 1** |
| Grupa z ręczną weryfikacją produkcji **po** wdrożeniu | **Grupa 13** |

**Odstępstwo od domyślnego limitu „~16-34 testy na funkcjonalność" jest świadome i wymuszone przez kryterium akceptacji 22**: cztery parsery × pięć obowiązkowych przypadków (`typical`, `no-date`, `no-stack`, `remote`, `unparsable`) to samo 20 testów. Limit 2-8 testów **na grupę** jest utrzymany bez wyjątku.

---

## Bramka danych — kiedy i kto dotyka produkcji

| Moment | Grupa | Czynność | Blokuje |
|---|---|---|---|
| **Przed napisaniem jakiegokolwiek pliku SQL** | **Grupa 1** | `SELECT url FROM possibilities LIMIT 5;`, `SELECT count(*) FROM possibilities WHERE url NOT LIKE 'http%';`, `SELECT DISTINCT category FROM possibilities;`, `SELECT current_schema();` | Grupy 2 i 3 — treść migracji i **istnienie** pliku `internal/database/data/001_normalize_urls.sql` zależą od wyniku |
| Po zbudowaniu całości | Grupa 13 | wykonanie runbooka `spec.md` §7.3 kroki 1-8 i weryfikacja kryteriów 3, 6, 7, 8, 12, 14 na żywej bazie | zamknięcie zadania |

Kolejność wymuszona przez dane, nie do przestawienia: **G1 (weryfikacja) → G2 (DDL) → G3 (seed + kroki danych)**, a wewnątrz runbooka **krok 3 (`001_normalize_urls.sql`) → krok 5 (DDL) → krok 6 (`002_backfill_source_id.sql`) → krok 7 (seed)**. Backfill dopasowuje wiersze po prefiksie `https://justjoin.it/%`, więc normalizacja URL-i musi się wykonać wcześniej, inaczej wiersze relatywne zostaną z `source_id IS NULL` na zawsze.

---

## Rozbieżności w specyfikacji do rozstrzygnięcia przed kodowaniem

Dwie sprzeczności wewnętrzne wykryte przy planowaniu. Żadnej **nie zgadujemy** — obie mają przypisanego właściciela i muszą zostać zamknięte jawną decyzją, zanim powstanie kod.

| ID | Rozbieżność | Właściciel | Rekomendacja planisty |
|---|---|---|---|
| **R1** | `spec.md` §5.3 wymaga, żeby `unseen` **pomijał wiersze z placeholderem** („`ExistingPossibilityUrls` zwraca wtedy tylko URL-e wierszy poprawnie sparsowanych"), a dwa akapity niżej i w §8.2 deklaruje `ExistingPossibilityUrls` jako **bez zmian**. Go-owe `unseen` operuje na samych URL-ach i nie zna `job_title`, więc bez zmiany w SQL-u nie da się pominąć placeholderów. | **Grupa 4**, krok 4.4 | Aneks A / H4 jest nadrzędny → dodać `WHERE job_title <> '(unparsed)'` (albo `<> @unparsed`) do `ExistingPossibilityUrls` i **poprawić brzmienie kryterium akceptacji 5**, które dziś mówi „bez zmian". Wariant alternatywny (osobne zapytanie zwracające URL-e placeholderów i różnica po stronie Go) jest droższy bez korzyści. |
| **R2** | Kryterium akceptacji 5 żąda, żeby linie `ON CONFLICT (url) DO UPDATE` i `SET posted_at = ...` były **niezmienione**, a §5.3 / H4 wymagają rozszerzenia `DO UPDATE SET` o siedem klauzul `CASE WHEN possibilities.job_title = '(unparsed)'`. §8.2 pokazuje jeszcze starą postać zapytania. | **Grupa 4**, krok 4.4; weryfikacja w **Grupie 14** | Aneks A / H4 jest nadrzędny → obowiązuje wersja z `CASE`. Kryterium 5 czytamy jako: **cel konfliktu `ON CONFLICT (url)` niezmieniony, kolumna konfliktu niezmieniona, `UNIQUE (url)` niezmienione; `DO UPDATE SET` rozszerzone wyłącznie o klauzule z H4**. Zakaz regresji dotyczy celu konfliktu, nie treści `SET`. |

---

## Kroki implementacji

### Grupa 1: Weryfikacja produkcji (bramka danych)

**Dependencies:** brak
**Files to Modify:** None — grupa wykonuje wyłącznie zapytania `SELECT` na bazie produkcyjnej i nie zapisuje żadnego pliku. Pole uzupełnione przez executora 2026-09-04, bo jego brak wymuszał degradację całego przebiegu do trybu sekwencyjnego.

**Dependencies (zależności):** brak — **musi być pierwsza**
**Files to Modify (pliki do zmiany):** `.maister/tasks/development/2026-09-04-crawler-multi-portal-sources/implementation/production-verification.md` (nowy, zapis wyników). Zero zmian w kodzie, zero DDL.
**Estimated Steps:** 5
**Testy:** 3 zapytania weryfikacyjne o binarnym werdykcie (brak testów Go — ta grupa nie produkuje kodu)

- [ ] 1.0 Zamknąć bramkę danych przed napisaniem pierwszego pliku SQL
  - [ ] 1.1 Zapisać zestaw sprawdzeń i oczekiwane odpowiedzi **przed** wykonaniem czegokolwiek
    - W1: `SELECT url FROM possibilities LIMIT 5;` → oczekiwane URL-e absolutne `https://justjoin.it/...`
    - W2: `SELECT count(*) FROM possibilities WHERE url NOT LIKE 'http%';` → oczekiwane `0`
    - W3: `SELECT DISTINCT category FROM possibilities;` → oczekiwane **dokładnie** `Go`, `PHP`, `Software Architect`, `AI/ML`, `C/C++`
  - [ ] 1.2 Wykonać W1-W3 na produkcji, wyłącznie do odczytu, potokiem z hosta wzorem `deploy.sh:55`
    - `$DOCKER compose exec -T postgres sh -c 'psql -U "$POSTGRES_USER" "$POSTGRES_DB"' <<< '...'`
    - Zakaz jakiegokolwiek `DDL`/`UPDATE` w tej grupie
  - [ ] 1.3 Wykonać dodatkowo `SELECT current_schema(); SHOW search_path;`
    - Potwierdzenie, że obiekty aplikacji żyją w schemacie `wok`, a nie `public`
    - Wynik jest wejściem dla asercji schematu w `deploy.sh` (grupa 13)
  - [ ] 1.4 Zapisać surowe wyniki i werdykt w `implementation/production-verification.md`
  - [ ] 1.5 Rozstrzygnąć dwie decyzje wynikające z wyników
    - Decyzja A: `count(*) > 0` w W2 ⇒ plik `internal/database/data/001_normalize_urls.sql` **powstaje** i wchodzi do runbooka jako krok 3. `count(*) = 0` ⇒ plik **nie powstaje w ogóle** (`minimal-implementation.md` — nie tworzymy martwego pliku „na wszelki wypadek")
    - Decyzja B: W3 zwraca cokolwiek innego niż pięć oczekiwanych nazw ⇒ **STOP całego zadania** i powrót do decyzji `category-key-mismatch` ze `scope-clarifications.md`; założenie z §9.3 przestaje obowiązywać

**Acceptance Criteria:**
- Trzy odpowiedzi zapisane dosłownie, z datą i sposobem wykonania
- Decyzja A i decyzja B zapisane jawnie, z uzasadnieniem
- Na produkcji nie wykonano ani jednego `DDL` ani `UPDATE`
- Grupy 2 i 3 nie zostały rozpoczęte przed zamknięciem tej grupy

---

### Grupa 2: Schemat — DDL i migracja idempotentna

**Dependencies:** 1
**Files to Modify:** `internal/database/schema.sql`, `internal/database/migrations/001_multi_portal_sources.sql` (nowy)
**Estimated Steps:** 4
**Testy:** 4 weryfikacje na jednorazowym, lokalnym kontenerze Postgresa (nie `go test` — test integracyjny w suicie Go jest poza zakresem, `spec.md` §13.3; to jest weryfikacja wdrożeniowa)

- [ ] 2.0 Doprowadzić schemat do stanu docelowego dwiema drogami
  - [ ] 2.1 Zapisać 4 sprawdzenia przed napisaniem DDL
    - V1: baza zbudowana ze `schema.sql` — `\d sources`, `\d scrape_targets`, `\d possibility_html`, `\d possibilities` zgodne z `spec.md` §5.1
    - V2: baza w stanie „przed" (`schema.sql` z HEAD) + `001_multi_portal_sources.sql` daje `\d` **identyczne** z V1 (kryterium 1)
    - V3: powtórne uruchomienie migracji nie zwraca błędu i nie zmienia `\d` (idempotencja)
    - V4: `grep -rniE 'public\.|CREATE SCHEMA public|SCHEMA public' internal/database/ scripts/` → zero trafień (kryterium 2)
  - [ ] 2.2 Dopisać DDL do `internal/database/schema.sql`
    - Cztery obiekty dosłownie wg §5.1: `sources`, `scrape_targets`, `possibilities.source_id` + `possibilities_source_id_idx`, `possibility_html`
    - Wyłącznie **niekwalifikowane** nazwy obiektów; nigdzie literału `public`
    - `possibilities.source_id` **NULLABLE**, `REFERENCES sources (id)` bez `ON DELETE` (świadome `NO ACTION`)
    - Brak `updated_at`, brak osobnego indeksu na `scrape_targets.source_id` (pokryty przez `UNIQUE (source_id, url)`)
    - Reuse: konwencja `BIGSERIAL`/`TIMESTAMPTZ` z istniejącego `schema.sql`
  - [ ] 2.3 Napisać `internal/database/migrations/001_multi_portal_sources.sql`
    - **Wyłącznie DDL** — zero `INSERT`, zero `UPDATE` („Separate Schema and Data" z `migrations.md`, rozstrzygnięcie H2)
    - Idempotencja: `CREATE TABLE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`, guardy `DO $$ ... current_schema() ... END $$` wzorem `deploy.sh:62`
    - Plik jest **append-only** — po wdrożeniu nie wolno go edytować, kolejne zmiany dostają `002_...`
    - Zero komentarzy w stylu change-log (`commenting.md`)
  - [ ] 2.4 Uruchomić V1-V4 na jednorazowym wolumenie i zapisać wynik
    - `\d` porównać dosłownie (diff dwóch zrzutów), nie „na oko"

**Acceptance Criteria:**
- V1-V4 przechodzą; zrzuty `\d` z obu dróg są identyczne (kryterium 1)
- Żaden plik SQL nie zawiera literału `public` w kontekście kwalifikacji obiektu (kryterium 2)
- `possibilities.url TEXT NOT NULL UNIQUE` **nietknięte** — w szczególności nie zamienione na `UNIQUE (source_id, url)`
- Plik migracji zawiera wyłącznie DDL

---

### Grupa 3: Seed i kroki danych

**Dependencies:** 2 (a istnienie `data/001_normalize_urls.sql` dodatkowo bramkowane decyzją A z grupy 1)
**Files to Modify:** `internal/database/seed.sql` (nowy), `internal/database/data/001_normalize_urls.sql` (nowy, **warunkowy**), `internal/database/data/002_backfill_source_id.sql` (nowy), `docker-compose.yaml` (montowanie `seed.sql`; **plik współdzielony z grupą 13** — nie modyfikować obu grup równolegle)
**Estimated Steps:** 6
**Testy:** 4 weryfikacje na lokalnym Postgresie

- [ ] 3.0 Dostarczyć dane konfiguracyjne i jednorazowe kroki danych
  - [ ] 3.1 Zapisać 4 sprawdzenia przed napisaniem plików
    - S1: `SELECT count(*) FROM scrape_targets` = `23` **i** `SELECT count(*) FROM sources` = `4` (kryterium 3)
    - S2: powtórne uruchomienie `seed.sql` nie zmienia liczby wierszy i nie zwraca błędu (kryterium 4)
    - S3: `001_normalize_urls.sql` uruchomione na bazie z URL-ami absolutnymi **nie zmienia ani jednego wiersza** (guard `WHERE url LIKE '/%'`)
    - S4: `002_backfill_source_id.sql` uruchomione dwukrotnie daje ten sam wynik; wiersze o URL-u spoza `https://justjoin.it/%` zostają `source_id IS NULL`
  - [ ] 3.2 Napisać `internal/database/seed.sql`
    - Treść dosłownie wg `spec.md` §6: 4 wiersze `sources` + 23 wiersze `scrape_targets`
    - Zero DDL; `ON CONFLICT (slug) DO NOTHING` i `ON CONFLICT (source_id, url) DO NOTHING`
    - **`'https://nofluffjobs.com/pl/c++?sort=newest'` z literalnym `+`** — nie kodować jako `%2B%2B`
    - Rozkład: justjoin 5, nofluffjobs 7, bulldogjob 6, solidjobs 5
  - [ ] 3.3 Napisać `internal/database/data/001_normalize_urls.sql` — **tylko jeśli decyzja A z grupy 1 tego wymaga**
    - `UPDATE possibilities SET url = 'https://justjoin.it' || url WHERE url LIKE '/%';`
    - Jeśli decyzja A mówi „nie" — plik nie powstaje, a krok 3 runbooka odpada
  - [ ] 3.4 Napisać `internal/database/data/002_backfill_source_id.sql`
    - `UPDATE possibilities SET source_id = (SELECT id FROM sources WHERE slug = 'justjoin') WHERE source_id IS NULL AND url LIKE 'https://justjoin.it/%';`
    - Guard `LIKE` czyni krok samoweryfikującym i idempotentnym
  - [ ] 3.5 Dopisać montowanie `seed.sql` w `docker-compose.yaml`
    - `- ./internal/database/seed.sql:/docker-entrypoint-initdb.d/seed.sql:ro`
    - Kolejność alfabetyczna `schema.sql` < `seed.sql` jest poprawna bez prefiksów numerycznych
  - [ ] 3.6 Uruchomić S1-S4: raz na świeżym wolumenie (ścieżka `docker-entrypoint-initdb.d`), raz na bazie po migracji (ścieżka potoku z hosta)

**Acceptance Criteria:**
- S1-S4 przechodzą (kryteria 3, 4)
- Po `001_normalize_urls.sql` (jeśli powstał) `SELECT count(*) FROM possibilities WHERE url NOT LIKE 'http%'` = `0` (kryterium 7)
- Pliki `data/*.sql` zawierają wyłącznie `UPDATE`, plik migracji wyłącznie DDL, `seed.sql` wyłącznie `INSERT`
- Obie ścieżki (świeży wolumen, żywa baza) dają identyczną zawartość `sources` i `scrape_targets`

---

### Grupa 4: Warstwa `sqlc` i trzej konsumenci

**Dependencies:** 2 (`sqlc` czyta `schema.sql`)
**Files to Modify:** `internal/database/queries/sources.sql` (nowy), `internal/database/queries/possibilities.sql`, `internal/database/generated/**` (regeneracja), `internal/handlers/scraped_handler.go`, `internal/justjoin/persist.go`, `internal/router/routing_test.go`
**Estimated Steps:** 7
**Testy:** 2 asercje kompilacyjne + pełne `go test ./...` jako bramka regresji

- [ ] 4.0 Przenieść warstwę zapytań na model wieloportalowy bez regresji dedupu
  - [ ] 4.1 Dodać dwie asercje kompilacyjne **przed** regeneracją
    - `var _ PossibilityStore = (*database.Queries)(nil)` w `internal/handlers`
    - `var _ OfferStore = (*database.Queries)(nil)` w `internal/justjoin`
    - Reuse: idiom `var _ Querier = (*Queries)(nil)` już obecny w `internal/database/generated`
    - Powód: rozjazd wąskiego interfejsu z generowanym kodem ma wywalić się w miejscu deklaracji, a nie dopiero przy budowaniu testów
  - [ ] 4.2 Utworzyć `internal/database/queries/sources.sql`
    - `-- name: ListEnabledScrapeTargets :many` — treść dosłownie z §8.1
    - Jawna lista kolumn, `JOIN sources`, `WHERE st.enabled`, `ORDER BY s.slug, st.category, st.id`
  - [ ] 4.3 Zmienić `internal/database/queries/possibilities.sql`
    - `CreatePossibility`: dochodzi **wyłącznie** `source_id` jako `$10`; `source_id` **nie** trafia do `DO UPDATE SET`
    - `CreatePossibilityHtml :exec` z `ON CONFLICT (possibility_id) DO UPDATE SET html = EXCLUDED.html, fetched_at = now()`
    - `ListPossibilitiesByCategory`: `SELECT *` → jawna lista kolumn + `LEFT JOIN sources s ON s.id = p.source_id`, `s.name AS source_name`
    - `GetPossibility` i `StatsPossibilityByCategory` — **bez zmian**
  - [ ] 4.4 **Zamknąć R1 i R2 jawną decyzją, zanim powstanie wygenerowany kod**
    - R1: czy `ExistingPossibilityUrls` dostaje filtr `job_title <> '(unparsed)'` (rekomendacja: tak, Aneks A / H4 jest nadrzędny)
    - R2: potwierdzić rozszerzenie `DO UPDATE SET` o klauzule `CASE WHEN possibilities.job_title = '(unparsed)'` z §5.3
    - Zapisać obie decyzje wraz z konsekwencją dla brzmienia kryterium akceptacji 5
    - **Cel konfliktu `ON CONFLICT (url)` i `UNIQUE (url)` pozostają nietknięte w każdym wariancie**
  - [ ] 4.5 Zregenerować: `make sqlc` (nie `make sqlc-generate` — ten target nie istnieje)
  - [ ] 4.6 Naprawić **trzech** konsumentów
    - `internal/handlers/scraped_handler.go:14-16` — `PossibilityStore.ListPossibilitiesByCategory` zwraca `[]database.ListPossibilitiesByCategoryRow`
    - `internal/justjoin/persist.go:11-13` — `OfferStore` dostaje `CreatePossibilityHtml`
    - `internal/router/routing_test.go:118` — stub sklepu w teście
  - [ ] 4.7 Zweryfikować przez `go vet ./...` **i `go test ./...`**
    - **`go build ./...` nie wystarcza** — złamanie `routing_test.go:118` ujawnia się dopiero przy budowaniu testów (ustalenie M1)

**Acceptance Criteria:**
- `go vet ./...` i `go test ./...` przechodzą (nie samo `go build`)
- `git diff internal/database/queries/possibilities.sql` pokazuje: dodanie `source_id`/`$10`, dodanie `CreatePossibilityHtml`, jawną listę kolumn w `ListPossibilitiesByCategory` oraz rozszerzenie `DO UPDATE SET` z H4 — **i nic więcej**; `ON CONFLICT (url)` jako cel konfliktu niezmieniony (kryterium 5 po korekcie z R2)
- R1 i R2 zapisane jako rozstrzygnięte decyzje
- Obie asercje kompilacyjne obecne i spełnione

---

### Grupa 5: Rdzeń crawlera — kontrakty, seam pobranie/parsowanie, throttling

**Dependencies:** brak — **ścieżka równoległa, może startować razem z grupą 1**
**Files to Modify:** `internal/justjoin/fetcher.go` (interfejs `LinkCollector`, `absoluteURL`, helpery pakietu), `internal/justjoin/transformer.go` (interfejs `OfferParser`, `ParsedOffer`, `fetchOfferHTML`), `internal/justjoin/throttling.go` (nowy), `internal/justjoin/fetcher_test.go` (nowy), `internal/offer/placeholder.go` (nowy). **`fetcher.go` i `transformer.go` są współdzielone z grupą 6** — grupa 6 startuje dopiero po zamknięciu tej.
**Estimated Steps:** 9
**Testy:** 6 (`TestAbsoluteURL`, sześć przypadków tabelarycznych)

- [x] 5.0 Postawić kontrakty, na których oprą się cztery portale
  - [x] 5.1 Napisać `TestAbsoluteURL` w `internal/justjoin/fetcher_test.go`
    - href relatywny od korzenia (`/job-offers/x`)
    - href relatywny względem ścieżki listingu
    - href już absolutny (zwracany bez zmian)
    - **href z query stringiem i fragmentem — muszą zostać zachowane** (wariant A decyzji `url-canonicalization`, kryterium 23)
    - href z literalnym `+` w ścieżce (`/pl/c++`)
    - href pusty / niedający się rozwinąć → błąd, wywołujący pomija z logiem, **bez panic**
    - Reuse: idiom testów tabelarycznych ze `internal/justjoin/transformer_test.go`, nazwy podtestów jako zdania po angielsku
  - [x] 5.2 Utworzyć `internal/offer/placeholder.go` z jedną stałą `Unparsed = "(unparsed)"`
    - Osobny pakiet, żeby `cmd/web` nie wciągnął `chromedp` przez `internal/justjoin`
  - [x] 5.3 Utworzyć `internal/justjoin/throttling.go` z dwoma blokami `const`
    - `listingInterval = 4h`, `maxListingJitter = 1h`, `minOfferPause = 2min`, `maxOfferPause = 3min`
    - `maxOffersPerPass = 100`, `maxListingPages = 3`
    - Usunąć stare `frequency`/`maxFrequencyJitter` z `fetcher.go:16-17` i stałe z `transformer.go:20-21`
    - Globalna stała `listingTimeout` **znika** (rozstrzygnięcie M5) — budżet czasu należy do implementacji `LinkCollector`
  - [x] 5.4 Zadeklarować `LinkCollector` w `fetcher.go` i `OfferParser` w `transformer.go`
    - Dosłownie wg §9.1, wraz z komentarzami „dlaczego"; interfejsy eksportowane, bo `cmd/crawler/main.go` musi je nazwać
    - **Nie budować:** rejestru pluginów, auto-rejestracji, map selektorów z konfiguracji, trzeciego interfejsu na readiness
  - [x] 5.5 Zaimplementować `absoluteURL(listingURL, href string) (string, error)`
    - `url.Parse` + `(*url.URL).ResolveReference`; **bez** obcinania query stringu i fragmentu
    - Jedyny punkt normalizacji URL-a w całym systemie
  - [x] 5.6 Przenieść `scrollAndCollectLinks` (`fetcher.go:168-198`) i `collectOfferLinks` (`fetcher.go:202-212`) z metod `OfferFetcher` na funkcje pakietu sparametryzowane selektorem karty oferty
    - `orderedLinks` (`fetcher.go:144-160`) — bez zmian
  - [x] 5.7 Zaimplementować `fetchOfferHTML(browserCtx, offerURL, readySelector) (string, error)`
    - Własna zakładka przez `newTab` (`browser.go:47`), `chromedp.Navigate`, **ograniczone czasowo i nieblokujące** czekanie na `readySelector` (`htmlReadyTimeout`, 15-20 s), `chromedp.Evaluate("document.documentElement.outerHTML", &html)`
    - **Zakaz `chromedp.Text` dla pól obowiązkowych** — blokuje do `offerTimeout` i traci HTML dokładnie tam, gdzie jest najcenniejszy
    - Błąd zwracany wyłącznie, gdy nie udało się pobrać żadnego HTML-a; `%w` przy opakowaniu błędów chromedp
  - [x] 5.8 Zadeklarować `ParsedOffer` wg §9.3 (`Tech` rozdzielane `", "`, `PostedAt` ISO `YYYY-MM-DD` albo `""`)
  - [x] 5.9 Uruchomić **tylko** nowe testy: `go test ./internal/justjoin/ -run TestAbsoluteURL` oraz `go list -deps ./cmd/web | grep chromedp` (oczekiwane zero trafień)

**Acceptance Criteria:**
- Sześć przypadków `TestAbsoluteURL` przechodzi; query string i fragment zachowane (kryterium 23)
- `go list -deps ./cmd/web | grep chromedp` nie daje trafień (kryterium 24)
- Cztery stałe throttlingu w jednym pliku; `frequency` i `maxFrequencyJitter` już nie istnieją
- `fetchOfferHTML` nie używa `chromedp.Text` do pól obowiązkowych
- Zero nowych abstrakcji ponad dwa interfejsy o 1 i 2 metodach

---

### Grupa 6: Pipeline — fetcher, transformer, persist

**Dependencies:** 4, 5
**Files to Modify:** `internal/justjoin/fetcher.go`, `internal/justjoin/transformer.go`, `internal/justjoin/persist.go`, `internal/justjoin/transformer_test.go`, `internal/category/category.go`
**Estimated Steps:** 7
**Testy:** 6 (`TestUnseen` rozszerzony ×2, `TestOffersPerPassLimit` ×2, `TestTargetRotation` ×2)

- [ ] 6.0 Przestawić trzy etapy pipeline'u na model per portal
  - [ ] 6.1 Napisać 6 testów przed zmianą kodu
    - `TestUnseen`: istniejące przypadki zachowane + **scenariusz wieloportalowy** (ta sama nazwa oferty pod różnymi URL-ami = różne oferty) + zachowanie dla wierszy `(unparsed)` zgodnie z decyzją R1
    - `TestOffersPerPassLimit`: licznik `scrapedInPass` zeruje się przy zmianie `Pass`; batche ponad `maxOffersPerPass` są pomijane z jednorazowym logiem
    - `TestTargetRotation`: przebieg `N` zaczyna od targetu o indeksie `N mod len(targets)`; rotacja jest bezstanowa i nie gubi żadnego targetu
    - Stuby ręczne, bez `testify`
  - [ ] 6.2 Zadeklarować typy `ScrapeTarget`, `FetchedOffers`, `ScrapedOffer` wg §9.4
    - `FetchedOffers` niesie **oba** klucze kategorii: `CategorySlug` (tylko do logów) i `CategoryName` (jedyne źródło `possibilities.category`)
  - [ ] 6.3 Przepisać `OfferFetcher`
    - `NewOfferFetcher(portalSlug string, targets []ScrapeTarget, collector LinkCollector) *OfferFetcher`
    - `Pass++` na każdy przebieg, rotacja kolejności targetów `N mod len(targets)` (rozstrzygnięcie M7)
    - Sekwencyjne przejście targetów, `collector.CollectOfferURLs(browserCtx, target.URL)`, wysyłka na własny kanał `FetchedUrls` (bufor 100) w `select` z `ctx.Done()`
    - `pause(listingInterval + rand.N(maxListingJitter))` — mechanizm losowania bez zmian
    - Prefiks loga `[<portal>/<kategoria> …]` w **każdej** linii
  - [ ] 6.4 Usunąć `ScrapeURL` i `scrapeParams` z `internal/category/category.go`
    - `All`, `Slug`, `Name`, `BySlug` bez zmian; ripple zerowy (`templates/parse.go:13`, `handlers/helper.go:79`, `stats_handler.go:35`, `scraped_handler.go:37` używają tylko `Slug`/`Name`)
    - Zaktualizować komentarz typu `Category`, który dziś opisuje `ScrapeURL`
  - [ ] 6.5 Przepisać `OfferTransformer`
    - `NewOfferTransformer(store, portalSlug string, parser OfferParser)`
    - `unseen` (`transformer.go:100`) — zachowanie zgodne z decyzją R1; `knownOffersTimeout` **5 s → 30 s** (H5)
    - `fetchOfferHTML(...)` → `parser.ParseOffer(html, url)`; błąd ⇒ `JobTitle = offer.Unparsed`, `CompanyName = offer.Unparsed`, reszta pusta, **HTML mimo to zapisany** (F5)
    - `CreatePossibilityParams.Category` zasilane **wyłącznie** z `batch.CategoryName`; slug nie ma drogi do tej kolumny
    - Kanał `Offers` zmienia typ na `chan ScrapedOffer`; `pause(minOfferPause + rand.N(maxOfferPause-minOfferPause))`
  - [ ] 6.6 Przepisać `OfferPersist`
    - `NewOfferPersist(store, portalSlug string)`; `Persist(<-chan ScrapedOffer)`
    - Timeout zapisu **3 s → 60 s**; dwa `INSERT`-y bez transakcji (`WithTx` zostaje nieużywane)
    - `CreatePossibility` → `saved.ID` → `CreatePossibilityHtml(saved.ID, html)`; błąd drugiego `INSERT`-u = log, oferta zostaje
    - Opakowywanie błędów `%w`
  - [ ] 6.7 Uruchomić **tylko** testy tej grupy: `go test ./internal/justjoin/ -run 'TestUnseen|TestOffersPerPassLimit|TestTargetRotation'`

**Acceptance Criteria:**
- Sześć testów tej grupy przechodzi; `TestOptDate` z `transformer_test.go` nadal przechodzi bez zmian
- Odstęp między ofertami losowany z `[2min, 3min)`, między przebiegami listingu z `[4h, 5h)` (kryterium 12)
- `scrapedInPass` nigdy nie przekracza 100 na przebieg per portal (kryterium 13)
- Każda oferta zapisana nowym pipeline'em ma wiersz w `possibility_html` (kryterium 14); oferta nieparsowalna ma `job_title = '(unparsed)'` **oraz** niepusty HTML (kryterium 15)
- `possibilities.category` dostaje wyłącznie `Name` (kryterium 6, weryfikacja produkcyjna w grupie 13)
- Każda linia loga crawlera zawiera slug portalu

---

### Grupa 7: Portal justjoin.it (przepisanie)

**Dependencies:** 5
**Files to Modify:** `internal/justjoin/justjoin_portal.go` (nowy), `internal/justjoin/justjoin_portal_test.go` (nowy), `internal/justjoin/testdata/justjoin/{typical,no-date,no-stack,remote,unparsable}.html` (nowe)
**Estimated Steps:** 6
**Testy:** 5 (`TestJustjoinParseOffer`, pięć przypadków)

- [ ] 7.0 Przenieść istniejący scraping justjoin.it na nowe kontrakty
  - [ ] 7.1 Rozpoznanie DOM-u — **zamknięcie kwestii otwartej 3 dla tego portalu**
    - Ustalić `ReadySelector()` (kandydat naturalny: `h1`, którego dzisiejszy kod używa jako niejawnego sygnału gotowości)
    - Ustalić selektory: tytuł, firma, lokalizacja, praca zdalna, stack, data publikacji; ustalić format daty na stronie
    - Potwierdzić selektor karty oferty dla infinite scrolla
    - Wynik zapisać jako komentarze „dlaczego" w pliku portalu
  - [ ] 7.2 Pozyskać 5 fixture'ów do `testdata/justjoin/`
    - **Dokładnie tą samą drogą co etap 1** (`document.documentElement.outerHTML`), żeby fixture i produkcja widziały ten sam HTML
    - `unparsable.html` syntetyczny (strona błędu albo szkielet SPA bez hydracji)
  - [ ] 7.3 Napisać `TestJustjoinParseOffer` — 5 przypadków tabelarycznych
    - typowa (wszystkie pola), brak daty, brak stacku, oferta zdalna, nieparsowalna (błąd)
    - Asercje: `Tech` rozdzielane `", "`; `PostedAt` w ISO albo `""`; brak pola opcjonalnego = wartość zerowa, **nie** błąd; brak pola obowiązkowego = błąd
  - [ ] 7.4 Zaimplementować `JustjoinParser` + `NewJustjoinParser() *JustjoinParser`
    - Czysta funkcja: zero I/O, zero zegara, zero sieci, zero `chromedp`
    - Normalizacja daty **w Go** — składanie stringów w JS (`transformer.go:218-224`) znika
    - Log braku pola opcjonalnego z nazwą portalu i URL-em (koniec cichego `""`)
    - **Anty-wzorce zakazane:** `strings.Contains(body, "Remote")`, lokalizacja jako „linia po tytule w `innerText`"
  - [ ] 7.5 Zaimplementować `JustjoinCollector` + `NewJustjoinCollector() *JustjoinCollector`
    - Infinite scroll przez helpery pakietu z kroku 5.6; **własna zakładka** przez `newTab` z lokalnym budżetem czasu
    - `absoluteURL(...)` na każdy href; href nierozwijalny pomijany z logiem
    - **`slices.Reverse` — TAK** (listing posortowany newest-first przez `?orderBy=DESC&sortBy=newest`)
  - [ ] 7.6 `go test ./internal/justjoin/ -run TestJustjoinParseOffer`

**Acceptance Criteria:**
- 5 testów przechodzi (część kryterium 22)
- Stary `scrapeOffer` w starym stylu **nie zostaje** — dwa równoległe style w pakiecie są niedopuszczalne
- Plik nazwany `snake_case`, konstruktory w postaci `New<X>() *X` (`coding-style.md`)

---

### Grupa 8: Portal nofluffjobs.com

**Dependencies:** 5
**Files to Modify:** `internal/justjoin/nofluffjobs_portal.go` (nowy), `internal/justjoin/nofluffjobs_portal_test.go` (nowy), `internal/justjoin/testdata/nofluffjobs/{typical,no-date,no-stack,remote,unparsable}.html` (nowe)
**Estimated Steps:** 6
**Testy:** 5 (`TestNofluffjobsParseOffer`)

- [ ] 8.0 Zbudować obsługę nofluffjobs.com od zera
  - [ ] 8.1 Rozpoznanie DOM-u — **zamknięcie kwestii otwartych 2 i 3 dla tego portalu**
    - Ustalić mechanikę listingu: infinite scroll czy paginacja; jeśli paginacja — kształt URL-a kolejnej strony
    - Ustalić selektor karty oferty, `ReadySelector()`, selektory pól i format daty
    - Zweryfikować na żywym listingu, że `?sort=newest` faktycznie zmienia kolejność (przesłanka doboru URL-i)
    - Wynik zapisać jako komentarze „dlaczego" w pliku portalu
  - [ ] 8.2 Pozyskać 5 fixture'ów do `testdata/nofluffjobs/` drogą etapu 1
  - [ ] 8.3 Napisać `TestNofluffjobsParseOffer` — 5 przypadków (jak w grupie 7)
  - [ ] 8.4 Zaimplementować `NofluffjobsParser` + `NewNofluffjobsParser()`
  - [ ] 8.5 Zaimplementować `NofluffjobsCollector` + `NewNofluffjobsCollector()`
    - Własna zakładka i własny budżet czasu na jednostkę pracy
    - **`slices.Reverse` — TAK**
    - URL-e targetów zawierają literalny `+` (`/pl/c++?sort=newest`) — **nie kodować** ani nie normalizować tego znaku
  - [ ] 8.6 `go test ./internal/justjoin/ -run TestNofluffjobsParseOffer`

**Acceptance Criteria:**
- 5 testów przechodzi (część kryterium 22)
- Mechanika listingu i `ReadySelector` udokumentowane w kodzie z uzasadnieniem
- URL zapisywany do bazy jest bit w bit URL-em z listingu po `absoluteURL`, nigdy `location.href`

---

### Grupa 9: Portal bulldogjob.pl

**Dependencies:** 5
**Files to Modify:** `internal/justjoin/bulldogjob_portal.go` (nowy), `internal/justjoin/bulldogjob_portal_test.go` (nowy), `internal/justjoin/testdata/bulldogjob/{typical,no-date,no-stack,remote,unparsable}.html` (nowe)
**Estimated Steps:** 6
**Testy:** 5 (`TestBulldogjobParseOffer`)

- [ ] 9.0 Zbudować obsługę bulldogjob.pl z paginacją
  - [ ] 9.1 Rozpoznanie DOM-u i paginacji — **zamknięcie kwestii otwartych 2 i 3 dla tego portalu**
    - **Nieznany kształt URL-a strony 2 i 3** — ustalić empirycznie i udokumentować
    - Ustalić selektor karty oferty, `ReadySelector()`, selektory pól i format daty
    - Potwierdzić brak parametru sortowania (konsekwencja: kolejność listingu arbitralna)
  - [ ] 9.2 Pozyskać 5 fixture'ów do `testdata/bulldogjob/` drogą etapu 1
  - [ ] 9.3 Napisać `TestBulldogjobParseOffer` — 5 przypadków
  - [ ] 9.4 Zaimplementować `BulldogjobParser` + `NewBulldogjobParser()`
  - [ ] 9.5 Zaimplementować `BulldogjobCollector` + `NewBulldogjobCollector()`
    - Paginacja: **własna zakładka na każdą stronę**, każda z pełnym budżetem czasu (rozstrzygnięcie M5)
    - `maxListingPages = 3` na target na przebieg; po limicie log `[bulldogjob] page limit (3) reached for <url>`
    - Limit stron jest **niezależny** od limitu 100 ofert na przebieg
    - **`slices.Reverse` — NIE** (listing nie jest posortowany newest-first)
  - [ ] 9.6 `go test ./internal/justjoin/ -run TestBulldogjobParseOffer`

**Acceptance Criteria:**
- 5 testów przechodzi (część kryterium 22)
- Kolektor nie przekracza 3 stron na target i loguje osiągnięcie limitu
- Kształt URL-a strony 2 i 3 udokumentowany w kodzie

---

### Grupa 10: Portal solid.jobs

**Dependencies:** 5
**Files to Modify:** `internal/justjoin/solidjobs_portal.go` (nowy), `internal/justjoin/solidjobs_portal_test.go` (nowy), `internal/justjoin/testdata/solidjobs/{typical,no-date,no-stack,remote,unparsable}.html` (nowe)
**Estimated Steps:** 6
**Testy:** 5 (`TestSolidjobsParseOffer`)

- [ ] 10.0 Zbudować obsługę solid.jobs z ochroną przed pułapką przekierowania
  - [ ] 10.1 Rozpoznanie DOM-u — **zamknięcie kwestii otwartych 2 i 3 dla tego portalu**
    - Ustalić mechanikę listingu, selektor karty oferty, `ReadySelector()`, selektory pól i format daty
    - Zweryfikować zachowanie przekierowania na adres kanoniczny oferty
  - [ ] 10.2 Pozyskać 5 fixture'ów do `testdata/solidjobs/` drogą etapu 1
  - [ ] 10.3 Napisać `TestSolidjobsParseOffer` — 5 przypadków
  - [ ] 10.4 Zaimplementować `SolidjobsParser` + `NewSolidjobsParser()`
  - [ ] 10.5 Zaimplementować `SolidjobsCollector` + `NewSolidjobsCollector()`
    - **ZAKAZ zapisywania `location.href` po nawigacji.** solid.jobs przekierowuje na adres kanoniczny; zapisany URL musi pochodzić z listingu, inaczej `unseen` przestaje trafiać i **każdy przebieg scrapuje wszystko od nowa** — przy pauzie 2-3 min prosta droga do bana
    - **`slices.Reverse` — NIE**
  - [ ] 10.6 `go test ./internal/justjoin/ -run TestSolidjobsParseOffer`

**Acceptance Criteria:**
- 5 testów przechodzi (część kryterium 22)
- W kodzie kolektora nie występuje odczyt `location.href` ani `chromedp.Location`
- URL zapisywany do bazy jest bit w bit tym samym stringiem, który trafia do `ExistingPossibilityUrls`

---

### Grupa 11: Wiązanie czterech pipeline'ów w `cmd/crawler/main.go`

**Dependencies:** 4, 6, 7, 8, 9, 10
**Files to Modify:** `cmd/crawler/main.go`, `internal/justjoin/targets.go` (nowy), `internal/justjoin/targets_test.go` (nowy)
**Estimated Steps:** 7
**Testy:** 4 (`TestGroupTargetsBySource`)

- [ ] 11.0 Uruchomić cztery niezależne pipeline'y sterowane danymi z bazy
  - [ ] 11.1 Napisać 4 testy czystej funkcji grupującej
    - pusta lista targetów → zero grup, sygnał do loga ostrzegawczego
    - slug portalu spoza czterech zaimplementowanych → target pominięty, bez panic
    - `scrape_targets.category` spoza `category.All` → target pominięty, bez panic
    - poprawne wejście → grupy per portal z rozwiązanym `CategoryName`
  - [ ] 11.2 Zaimplementować `groupTargetsBySource` w `internal/justjoin/targets.go`
    - Czysta funkcja `[]database.ListEnabledScrapeTargetsRow → map[string][]ScrapeTarget` + lista pominięć
    - Walidacja kategorii przez `category.BySlug` **przy starcie** (`validation.md`, `error-handling.md` → „Graceful Degradation")
  - [ ] 11.3 Wpiąć `db.ListEnabledScrapeTargets(ctx)` na starcie `main()`
    - Błąd zapytania → log + `panic` (spójnie z dzisiejszym zachowaniem przy braku bazy)
    - Zero włączonych targetów → **log ostrzegawczy** `no enabled scrape targets, crawler has nothing to do`, nie ciche milczenie
  - [ ] 11.4 Zbudować literalną listę czterech portali w `main()`
    - Allowlist w kodzie, nie dowolny slug z bazy; idiom wiązania z `cmd/web/main.go:38-58`
    - Portal bez włączonych targetów → log + pominięcie pipeline'u
    - **Bez rejestru, bez fabryki, bez refleksji**
  - [ ] 11.5 Uruchomić 3N goroutine'ów — po jednej gałęzi na portal
    - **Każda gałąź ma własne kanały `FetchedUrls` i `Offers`** — jeden właściciel, jeden zamykacz
    - Wariant „N fetcherów pisze do wspólnego kanału" jest odrzucony (dałby `panic: close of closed channel`)
  - [ ] 11.6 Zweryfikować graceful shutdown ręcznie
    - `SIGTERM` **w trakcie aktywnego scrapowania**: brak `panic: close of closed channel`, brak zawieszenia, `wg.Wait()` domyka 3N goroutine'ów
  - [ ] 11.7 `go test ./internal/justjoin/ -run TestGroupTargetsBySource` + ręczny scenariusz SIGTERM

**Acceptance Criteria:**
- 4 testy przechodzą
- Uruchomienie startuje cztery niezależne pipeline'y, każda linia loga zawiera slug portalu (kryterium 9)
- `SIGTERM` kończy proces czysto podczas scrapowania (kryterium 10)
- Pusta `scrape_targets` daje log ostrzegawczy; nieznany slug portalu i nieznany slug kategorii dają log + pominięcie, nie panic (kryterium 11)

---

### Grupa 12: UI — kolumna `Source` w liście zescrapowanych ofert

**Dependencies:** 4 (typ `ListPossibilitiesByCategoryRow`), 5 (`internal/offer.Unparsed`)
**Files to Modify:** `internal/handlers/scraped_handler.go`, `internal/templates/scraped/list.gohtml`, `internal/handlers/scraped_handler_test.go` (nowy)
**Visual References:**
- mockup: `analysis/design-context/ascii/ui-mockups.md`
  element: `screen:scraped-list`
  locator: sekcja `## 4. Mockup: stan docelowy (AFTER) {#after}`, wiersze 83-120
  acceptance: dokładnie **5 kolumn** w kolejności `Title, Source, Published, Technologies, URL`; `Source` na pozycji drugiej, bezpośrednio po `Title`; kontener tabeli bez zmian (`card shadow-sm` + `table-responsive` + `table table-striped table-hover align-middle mb-0`)
- mockup: `analysis/design-context/ascii/ui-mockups.md`
  element: `screen:scraped-list-before`
  locator: sekcja `## 3. Mockup: stan obecny (BEFORE) {#before}`, wiersze 54-82
  acceptance: punkt odniesienia dla diffa — `git diff internal/templates/scraped/list.gohtml` zmienia wyłącznie to, co odróżnia `#before` od `#after`; żadnych zmian w `head`, `header`, `categorynav`, `footer`
- mockup: `analysis/design-context/ascii/ui-mockups.md`
  element: `screen:scraped-list-empty`
  locator: sekcja `### 5.5 Pusty stan {#pusty-stan}`, wiersze 185-199
  acceptance: `colspan="4"` → **`colspan="5"`** w `list.gohtml:29`; komunikat „No scraped offers yet." wyśrodkowany względem całej tabeli, klasy `text-center text-muted py-5` bez zmian
- mockup: `analysis/design-context/ascii/ui-mockups.md`
  element: `screen:category-nav`
  locator: sekcja `## 7. Ocena categorynav.gohtml {#categorynav}`, wiersze 219-239
  acceptance: **werdykt „bez zmian"** — `git diff internal/templates/partials/categorynav.gohtml` pusty
- mockup: `analysis/design-context/ascii/ui-mockups.md`
  element: `component:source-column-header`
  locator: sekcja `#after`, blok `<thead>`
  acceptance: nowy `<th>Source</th>` bezpośrednio po `<th>Title</th>` (po linii 13 `list.gohtml`)
- mockup: `analysis/design-context/ascii/ui-mockups.md`
  element: `component:source-cell`
  locator: sekcja `#after`, blok `<tbody>` + sekcja `## 8. {#reuse}`, wiersze 240-284
  acceptance: `<td class="text-nowrap">` po linii 22, treść `<span class="badge text-bg-light border">{{ .Source }}</span>`; klasa `text-nowrap` obowiązkowa, żeby `nofluffjobs.com` się nie łamał; **zero nowego CSS**, badge z `offers/list.gohtml:40,44`
- mockup: `analysis/design-context/ascii/ui-mockups.md`
  element: `component:possibility-row`
  locator: sekcja `### 5.1 Wiersz normalny {#wiersz-normalny}`, wiersze 123-130
  acceptance: `possibilityRow` (`scraped_handler.go:28-33`) dostaje `Source string` **i** `Unparsed bool`; kolejność pól renderu zgodna z kolejnością kolumn
- mockup: `analysis/design-context/ascii/ui-mockups.md`
  element: `component:row-no-published-date`
  locator: sekcja `### 5.2 Wiersz bez daty publikacji {#wiersz-bez-daty}`, wiersze 131-144
  acceptance: **zmiana niewymagana** — `formatDate` (`helper.go:36-41`) już zwraca `-`; potwierdzić testem, nie kodem
- mockup: `analysis/design-context/ascii/ui-mockups.md`
  element: `component:row-duplicate-across-sources`
  locator: sekcja `### 5.3 Duplikat: ta sama oferta z dwóch portali {#wiersz-duplikat}`, wiersze 145-160
  acceptance: dwa wiersze o tym samym tytule i różnych portalach są rozróżnialne bez przewijania w poziomie — to ten wariant przesądza o umieszczeniu `Source` **obok** `Title`
- mockup: `analysis/design-context/ascii/ui-mockups.md`
  element: `component:row-unparsed-placeholder`
  locator: sekcja `### 5.4 Wiersz z placeholderem po nieudanym parsowaniu {#wiersz-placeholder}`, wiersze 161-184
  acceptance: **wariant B** — komórka tytułu renderuje `text-muted fst-italic` zamiast `fw-medium`, gdy `.Unparsed`; źródłem prawdy jest `internal/offer.Unparsed`, **nie** literał `"(unparsed)"` w szablonie
- mockup: `analysis/design-context/ascii/ui-mockups.md`
  element: `component:source-null-fallback`
  locator: sekcja `## 8. {#reuse}`, wiersze 240-284
  acceptance: `source_id IS NULL` renderuje **gołe `-`, bez elementu `<span class="badge">`**; idiom `{{ if eq .Source "-" }}-{{ else }}…{{ end }}` z `offers/list.gohtml:36-45`
- mockup: `analysis/design-context/ascii/ui-mockups.md`
  element: `component:empty-state-row`
  locator: sekcja `### 5.5 Pusty stan {#pusty-stan}`, wiersze 185-199
  acceptance: jedyna komórka wiersza pustego stanu ma `colspan="5"`

**Estimated Steps:** 8
**Testy:** 5

- [ ] 12.0 Dodać kolumnę `Source` bez jednej linii nowego CSS-a
  - [ ] 12.1 Napisać 5 testów przed zmianą
    - handler mapuje `SourceName` na `Source` z wartością nazwy portalu (`nofluffjobs.com`)
    - `SourceName` `NULL` (`pgtype.Text{Valid:false}`) → `Source` = `-`
    - `JobTitle == offer.Unparsed` → `Unparsed == true`
    - render szablonu daje 5 nagłówków w kolejności `Title, Source, Published, Technologies, URL`
    - render pustej listy daje jedną komórkę z `colspan="5"`
    - Stub `PossibilityStore` pisany ręcznie, idiom ze stubów w `transformer_test.go` i `routing_test.go`
  - [ ] 12.2 Rozszerzyć `possibilityRow` o `Source string` i `Unparsed bool`
  - [ ] 12.3 Zmienić typ w `PossibilityStore` i mapowanie w `List`
    - `Source: formatText(p.SourceName)` — `-` dla `NULL`, spójnie z resztą aplikacji (`helper.go:64`)
    - `Unparsed: p.JobTitle == offer.Unparsed`
    - **Nazwa portalu pochodzi z `LEFT JOIN sources`** w zapytaniu, nie z mapy w handlerze (mapa byłaby cache'em nieodświeżalnym bez restartu, a źródła zmienia się przez `psql`)
  - [ ] 12.4 Dodać `<th>Source</th>` po `<th>Title</th>` w `scraped/list.gohtml`
  - [ ] 12.5 Dodać `<td class="text-nowrap">` z rozróżnieniem badge / myślnik
  - [ ] 12.6 Zmienić klasę komórki tytułu na warunkową (`fw-medium` vs `text-muted fst-italic`)
  - [ ] 12.7 Podnieść `colspan="4"` → `colspan="5"` w wierszu pustego stanu (`list.gohtml:29`)
  - [ ] 12.8 Uruchomić **tylko** testy handlera + `git diff --stat internal/templates/`

**Acceptance Criteria:**
- 5 testów przechodzi
- `/scr4p3d/go` renderuje pięć kolumn w kolejności `Title, Source, Published, Technologies, URL` (kryterium 16)
- Wiersz z `source_id IS NULL` pokazuje gołe `-` bez `<span class="badge">` (kryterium 17)
- Wiersz `(unparsed)` ma komórkę tytułu z `text-muted fst-italic` (kryterium 18)
- Pusta kategoria renderuje `colspan="5"` (kryterium 19)
- `git diff internal/templates/` nie dodaje ani jednej reguły CSS, ani jednego pliku JS, ani jednego partiala (kryterium 20)
- `/scr4p3d/{category}` i `/st4ts/{category}` odpowiadają `200` dla wszystkich pięciu kategorii (kryterium 8)
- Każde kryterium `acceptance` z sekcji `Visual References` powyżej sprawdzone pojedynczo

---

### Grupa 13: Runbook wdrożenia i weryfikacja produkcyjna

**Dependencies:** 3, 11, 12
**Files to Modify:** `scripts/deploy.sh`, `docker-compose.yaml` (`mem_limit: 4g`; **plik współdzielony z grupą 3**)
**Estimated Steps:** 8
**Testy:** brak automatycznych — 6 weryfikacji na żywej bazie po wdrożeniu

- [ ] 13.0 Wdrożyć całość w kolejności wymuszonej przez dane
  - [ ] 13.1 Zapisać listę weryfikacji **przed** wdrożeniem
    - P1: `SELECT count(*) FROM scrape_targets` = `23`, `SELECT count(*) FROM sources` = `4`
    - P2: `SELECT DISTINCT category FROM possibilities` = pięć nazw, żadnego sluga
    - P3: `SELECT count(*) FROM possibilities WHERE url NOT LIKE 'http%'` = `0`
    - P4: `/scr4p3d/{category}` i `/st4ts/{category}` = `200` dla pięciu kategorii, **przed i po** backfillu
    - P5: zapytanie o sieroty z kryterium 14 (z odcięciem po `created_at` względem wiersza `sources`) = `0`
    - P6: znaczniki czasu w logu potwierdzają `[2min, 3min)` między ofertami i `[4h, 5h)` między przebiegami
  - [ ] 13.2 Dodać asercję schematu w `deploy.sh` **przed** blokiem DDL
    - Blok `DO $$ … IF current_schema() IS NULL THEN RAISE EXCEPTION … RAISE NOTICE 'applying DDL to schema %' … END $$;` wg §7.3 krok 4
  - [ ] 13.3 Wpiąć pliki migracji i danych **potokiem z hosta**, nie przez `psql -f`
    - Katalog `internal/database/migrations/` **nie jest montowany** do kontenera — `psql -f` go nie znajdzie (rozstrzygnięcie H1)
    - Wzorzec: `$DOCKER compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" "$POSTGRES_DB"' < <plik>`
  - [ ] 13.4 Ułożyć kroki w `deploy.sh` dokładnie w kolejności z §7.3
    - `make db-backup` (istnieje, `deploy.sh:24-29`) → **warunkowo** `data/001_normalize_urls.sql` → asercja schematu → `migrations/001_multi_portal_sources.sql` → `data/002_backfill_source_id.sql` → `seed.sql` → `docker compose up -d`
    - **Krok 3 przed krokiem 6** — backfill dopasowuje po prefiksie `https://justjoin.it/%`
    - Krok warunkowy bramkowany warunkiem w skrypcie, **nie** interaktywnym pytaniem; `set -euo pipefail` zachowane
  - [ ] 13.5 Dodać `mem_limit: 4g` do usługi `crawler` w `docker-compose.yaml`
    - **Nie dodawać `disable-dev-shm-usage`** — flaga jest już w `DefaultExecAllocatorOptions` chromedp (ustalenie M2); dopisanie byłoby martwym kodem
    - `shm_size` również niepotrzebne; `init: true` już ustawione (`docker-compose.yaml:54`)
  - [ ] 13.6 Wykonać wdrożenie na produkcji
    - **Nie używać `make db-reset`** — `Makefile:55` robi `DROP SCHEMA public CASCADE`, co na VPS-ie ze schematem `wok` nic nie usuwa, a potem `schema.sql` wywala się na `relation "possibilities" already exists`
  - [ ] 13.7 Wykonać P1-P6 i zapisać wyniki
  - [ ] 13.8 Odnotować przyjęte długi w dokumentacji zadania
    - `possibility_html` write-only bez retencji (rosnący `pg_dump`), brak rollbacku poza `pg_dump`, brak transakcji przy dwóch `INSERT`-ach, `make db-reset` niebezpieczny na VPS

**Acceptance Criteria:**
- P1-P6 przechodzą na produkcji (kryteria 3, 6, 7, 8, 12, 14)
- Kolejność kroków 1-8 z §7.3 zachowana; krok normalizacji URL-i wykonany przed backfillem
- Powtórne uruchomienie `deploy.sh` na tej samej bazie nie zwraca błędu i nie zmienia liczby wierszy (kryterium 4)
- `deploy.sh` pozostaje nieinteraktywny

---

### Grupa 14: Przegląd testów, luki i mapowanie kryteriów akceptacji

**Dependencies:** wszystkie poprzednie
**Files to Modify:** `internal/justjoin/*_test.go`, `internal/handlers/scraped_handler_test.go`
**Estimated Steps:** 6
**Testy:** max 10 dodatkowych

- [ ] 14.0 Domknąć pokrycie i potwierdzić wszystkie 24 kryteria
  - [ ] 14.1 Przejrzeć 41 testów z grup 5-12
  - [ ] 14.2 Przeanalizować luki **wyłącznie dla tej zmiany**
    - Kandydaci: mapowanie `ParsedOffer` → `CreatePossibilityParams` (czy `CategoryName`, a nie slug, trafia do `Category`), mapowanie polityki placeholdera, format `Tech` rozdzielany `", "` zgodny z `string_to_array(tech, ',')` w `StatsPossibilityByCategory`, `TestOptDate` jako regresja
    - **Świadomie niepokryte** (nie uzupełniać): `ON CONFLICT (url)` przeciw realnemu Postgresowi, kolektory wymagające żywego chromedp, zapis `possibility_html`, warstwa szablonów poza asercjami z grupy 12
  - [ ] 14.3 Dopisać **maksymalnie 10** testów strategicznych
  - [ ] 14.4 Uruchomić `go build ./...`, `go vet ./...`, `go test ./...`
  - [ ] 14.5 Uruchomić `go list -deps ./cmd/web | grep chromedp` (oczekiwane zero trafień)
  - [ ] 14.6 Przejść tabelę 24 kryteriów akceptacji i oznaczyć każde jako spełnione, z dowodem
    - Kryterium 5 czytane po korekcie z **R2**; zapisać, że rozbieżność została rozstrzygnięta, a nie przeoczona
    - Jawnie odnotować, że **zakaz regresji dedupu nie ma pokrycia automatycznego** — weryfikacja jest ręczna: `git diff` na `possibilities.sql` + P1-P3 z grupy 13

**Acceptance Criteria:**
- Wszystkie testy przechodzą; łącznie 41-51
- Dodano nie więcej niż 10 testów
- `go build ./...`, `go vet ./...`, `go test ./...` przechodzą (kryterium 21)
- `go list -deps ./cmd/web | grep chromedp` bez trafień (kryterium 24)
- Każde z 24 kryteriów ma przypisany dowód spełnienia

---

## Kolejność wykonania i równoległość

```
tor A (dane)                        tor B (kod, niezależny od bazy)
─────────────────────────────       ──────────────────────────────
G1  weryfikacja produkcji  ◄── bramka
      │
G2  DDL + migracja                  G5  rdzeń crawlera (kontrakty)
      │                                   │
   ┌──┴──────┐                        ┌───┼───┬───┬───┐
G3 seed   G4 sqlc                    G7  G8  G9  G10   (4 portale równolegle)
   │         │                        └───┴───┴───┴───┘
   │         └──────────┬──────────────────────┐   │
   │                  G6 pipeline              │   │
   │                    │                      │   │
   │                    └──────────┬───────────┴───┘
   │                             G11 wiązanie w main()
   │                               │
   │                  G12 UI (zależy od G4 i G5, równolegle z G6-G11)
   │                               │
   └───────────────┬───────────────┘
                 G13 runbook wdrożenia
                   │
                 G14 przegląd testów i kryteriów
```

**Można robić równolegle:**

| Zestaw | Warunek |
|---|---|
| **G5 z G1-G4** | G5 nie zależy od bazy ani od `sqlc` — kontrakty, `absoluteURL`, `throttling.go` i `internal/offer` są czystym Go |
| **G3 z G4** | oba zależą tylko od G2 i dotykają rozłącznych plików |
| **G7, G8, G9, G10** | cztery portale, cztery rozłączne pliki, wspólna zależność tylko od G5 |
| **G12 z G6 i G7-G11** | UI zależy od G4 i G5, dotyka `internal/handlers` i `internal/templates` — zero wspólnych plików z crawlerem |

**Zablokowane twardo:**

| Grupa | Blokada | Powód |
|---|---|---|
| G2, G3 | **G1** | treść migracji i **istnienie** `data/001_normalize_urls.sql` zależą od wyniku zapytań produkcyjnych; wymuszenie URL-a absolutnego jest nieodwracalne |
| G3 | G2 | seed wymaga istniejących tabel; `data/002` wymaga kolumny `source_id` |
| G4 | G2 | `sqlc` czyta `internal/database/schema.sql` |
| G6 | G4 **i** G5 | potrzebuje wygenerowanych typów **i** kontraktów; dzieli `fetcher.go`/`transformer.go` z G5 |
| G7-G10 | G5 | implementują interfejsy `LinkCollector`/`OfferParser` |
| G11 | G6 + G7 + G8 + G9 + G10 | `main()` nazywa `New<Portal>Collector`/`New<Portal>Parser` — brak któregokolwiek łamie kompilację |
| G13 | G3 + G11 + G12 | wdrażamy komplet, nie fragment |

**Pliki współdzielone (wymuszają szeregowanie, nie równoległość):**

| Plik | Grupy | Reguła |
|---|---|---|
| `internal/justjoin/fetcher.go` | 5, 6 | G6 startuje po zamknięciu G5 |
| `internal/justjoin/transformer.go` | 5, 6 | j.w. |
| `internal/justjoin/transformer_test.go` | 6, 14 | G14 jest ostatnia |
| `docker-compose.yaml` | 3, 13 | G13 jest po G3 |
| `internal/handlers/scraped_handler.go` | 4, 12 | G12 po G4 |
| `internal/database/schema.sql` | 2 (zapis), 4 (odczyt przez `sqlc`) | G4 po G2 |

---

## Mapowanie 24 kryteriów akceptacji na właścicieli

Żadne kryterium nie zostaje bez właściciela. „Weryfikacja" oznacza grupę, która potwierdza kryterium na żywym systemie.

| # | Kryterium (skrót) | Właściciel | Weryfikacja |
|---|---|---|---|
| 1 | `schema.sql` i migracja opisują ten sam stan | **G2** | G2 (V1/V2), G14 |
| 2 | Żaden plik SQL nie zawiera literału `public` | **G2** | G2 (V4), G14 |
| 3 | `scrape_targets` = 23, `sources` = 4 | **G3** | G3 (S1), G13 (P1) |
| 4 | Idempotencja migracji i seedu | **G3** | G3 (S2), G13 |
| 5 | `CreatePossibility` — cel konfliktu nietknięty (po korekcie R2) | **G4** | G4 (`git diff`), G14 |
| 6 | `DISTINCT category` = pięć nazw, żadnego sluga | **G6** | G13 (P2) |
| 7 | `count(*) WHERE url NOT LIKE 'http%'` = `0` | **G3** | G13 (P3) |
| 8 | `/scr4p3d/*` i `/st4ts/*` = `200` przed i po backfillu | **G12** | G13 (P4) |
| 9 | Cztery pipeline'y, slug portalu w każdej linii loga | **G11** | G13 |
| 10 | `SIGTERM` bez `panic: close of closed channel` | **G11** | G11 (11.6) |
| 11 | Pusta `scrape_targets` = log ostrzegawczy; nieznany slug = pominięcie | **G11** | G11 (11.1) |
| 12 | Odstępy `[2min, 3min)` i `[4h, 5h)` | **G6** (stałe w G5) | G13 (P6) |
| 13 | Max 100 ofert na przebieg per pipeline | **G6** | G6 (`TestOffersPerPassLimit`) |
| 14 | Brak sierot bez `possibility_html` (z odcięciem po `created_at`) | **G6** | G13 (P5) |
| 15 | Oferta nieparsowalna: `(unparsed)` + niepusty HTML | **G6** (polityka), G7-G10 (błąd parsera) | G13 |
| 16 | Pięć kolumn w kolejności `Title, Source, Published, Technologies, URL` | **G12** | G12, G13 |
| 17 | `source_id IS NULL` → gołe `-`, bez badge'a | **G12** | G12 |
| 18 | `(unparsed)` → `text-muted fst-italic` | **G12** | G12 |
| 19 | Pusty stan z `colspan="5"` | **G12** | G12 |
| 20 | Zero nowego CSS, JS i partiali | **G12** | G12 (`git diff --stat`) |
| 21 | `go build`, `go vet`, `go test ./...` przechodzą | **G14** | G14 |
| 22 | Każdy z czterech parserów ma 5 przypadków | **G7, G8, G9, G10** | G14 |
| 23 | `absoluteURL` pokrywa query string i fragment | **G5** | G5, G14 |
| 24 | `go list -deps ./cmd/web \| grep chromedp` bez trafień | **G5** (pakiet `internal/offer`) | G5, G14 |

---

## Zgodność ze standardami

Obowiązują standardy z `.maister/docs/standards/`, wskazane przez `.maister/docs/INDEX.md`:

| Standard | Egzekwowany w grupach |
|---|---|
| `global/minimal-implementation.md` | G5 (dwa interfejsy o 1-2 metodach, zero rejestrów i fabryk), G2 (brak `updated_at`, brak redundantnego indeksu), G3 (`data/001` nie powstaje, jeśli nie jest potrzebny), G11 (literalna lista czterech portali), G13 (brak `disable-dev-shm-usage` jako martwego kodu) |
| `global/coding-style.md` | G5, G7-G10 (pliki `snake_case`, typy `<Portal><Rola>`, konstruktory `New<X>() *X`, DRY: `absoluteURL` i stała placeholdera w jednym miejscu), G6 (`frequency` → `listingInterval`) |
| `global/error-handling.md` | G5, G6, G7-G10 (`fmt.Errorf("...: %w", err)`), G11 („Graceful Degradation" — nieznany slug = log + pominięcie), G13 („Fail Fast" — asercja `current_schema()` przed DDL) |
| `global/commenting.md` | wszystkie grupy — komentarze „dlaczego", **zero komentarzy w stylu change-log** („dodane w migracji 001", „było 3 s") |
| `global/validation.md` | G11 (walidacja slugów kategorii przy starcie), G2 (`CHECK`, `UNIQUE` — walidacja wielowarstwowa) |
| `global/conventions.md` | G11 („Interface-Based Dependency Injection" — wiązanie w `main()`), G5 (nowe pliki w istniejących katalogach) |
| `backend/migrations.md` | G2 („Descriptive Names", „Small and Focused", plik append-only), G3 („Separate Schema and Data") |
| `backend/models.md` | G2 (tabela uzasadnień z `spec.md` §5.2) |
| `backend/queries.md` | G4 („Parameterized Queries", „Avoid N+1", „Select Only Needed Columns", „Query Timeouts") |
| `backend/infrastructure.md` | G13 (`docker-compose.yaml`, zmienne środowiskowe, brak nowych zależności zewnętrznych) |
| `testing/test-writing.md` | G5-G12 (stdlib `testing`, tabele + `t.Run`, nazwy podtestów jako zdania po angielsku, ręczne stuby, bez `testify`) |

**Świadome odstępstwa** (wszystkie opisane w `spec.md` §12 i §15.2, nie do „naprawiania" w trakcie implementacji): brak rollbacku migracji, brak tabeli `schema_migrations`, brak transakcji przy dwóch `INSERT`-ach, brak testu integracyjnego dla dedupu po URL, throttling jako `const` zamiast `CRAWLER_*`, `possibilities.source_id` nullable.

---

## Notatki

- **Test-driven bez wyjątku**: każda grupa zaczyna się od zapisania sprawdzeń (testów Go albo zapytań weryfikacyjnych) i kończy ich uruchomieniem.
- **Uruchamiać tylko nowe testy** po każdej grupie (`go test -run <Nazwa>`), nie całą suitę — pełne `go test ./...` należy do G4 (bramka regresji po `sqlc`) i G14.
- **Reuse przed pisaniem**: `pause`, `rand.N`, `newBrowser`/`newTab`, `orderedLinks`, `optDate`/`optText`, `formatDate`/`formatText`, badge `text-bg-light border`, idiom `{{ if eq }}`, guard `current_schema()` z `deploy.sh:62`, `pg_dump` z `deploy.sh:24-29`.
- **Trzy rzeczy, które zabijają zadanie po cichu** — do sprawdzenia przy każdym przeglądzie: slug zamiast `Name` w `possibilities.category`, URL relatywny zamiast absolutnego, zapis `location.href` po przekierowaniu solid.jobs.
- Kwestie otwarte **5** (recykling Chromium) i **6** (rozmiar puli `pgxpool`) pozostają otwarte świadomie — to decyzje strojeniowe po pierwszym przebiegu produkcyjnym, nie zadania dla żadnej z grup.
