# Analiza Luk: Crawler — dodanie portali NoFluffJobs, Bulldogjob, solid.jobs

**Data**: 2026-09-04
**Wejście**: `analysis/codebase-analysis.md`, `analysis/clarifications.md`, `analysis/approved-urls.md`
**Dokumenty projektu**: `.maister/docs/INDEX.md`, `project/vision.md`, `project/tech-stack.md`
**Standardy**: `standards/backend/{migrations,models,queries}.md`, `standards/global/{minimal-implementation,conventions,error-handling,coding-style}.md`, `standards/testing/test-writing.md`

## Podsumowanie

- **Poziom ryzyka**: Wysoki
- **Szacowany nakład**: Wysoki
- **Wykryte charakterystyki**: `modifies_existing_code`, `creates_new_entities`, `involves_data_operations`, `ui_heavy` (częściowo)
- **Typ zmiany**: mieszany — `additive` w schemacie (nowe tabele + kolumna, bez ruszania `UNIQUE (url)`), `refactor-based` w `internal/crawler` (rozcięcie `scrapeOffer` na seam pobranie/parsowanie bez zmiany zachowania dla justjoin.it), `modificative` w throttlingu i w kolejności listingu.
- **Wymagania kompatybilności**: **strict** dla ścieżki dedupu po URL i dla istniejących widoków `/scr4p3d/{category}` i `/st4ts/{category}`; moderate dla reszty.

Kluczowe ustalenie tej fazy, którego nie było w analizie kodu: **`possibilities.category` przechowuje nazwę wyświetlaną (`Name`), a nie slug**. Crawler zapisuje `cat.Name` (`fetcher.go:73`), a oba handlery web odpytują po `cat.Name` (`scraped_handler.go:55`, `stats_handler.go:53`). Tymczasem `analysis/approved-urls.md` kluczuje 21 wierszy `scrape_targets` po slugach (`go`, `php`, `architect`, `ai-ml`, `c`). Jeżeli crawler zacznie zapisywać slug, **wszystkie istniejące wiersze znikną z UI bez żadnego błędu** — listy po prostu pokażą „No scraped offers yet.".

---

## Charakterystyki zadania

| Charakterystyka | Wartość | Dowód |
|---|---|---|
| `has_reproducible_defect` | **nie** | Brak opisu awarii, stack trace'u ani scenariusza błędu. To rozbudowa, nie naprawa. |
| `modifies_existing_code` | **tak** | Zmieniane: `internal/crawler/{fetcher,transformer,persist}.go`, `internal/category/category.go`, `internal/database/{schema.sql,queries/possibilities.sql}`, `cmd/crawler/main.go`, `scripts/deploy.sh`. |
| `creates_new_entities` | **tak** | Nowe encje: `sources`, `scrape_targets`, `possibility_html`, `possibilities.source_id`; nowe pliki ekstraktorów per portal. |
| `involves_data_operations` | **tak** | CREATE/READ na `possibilities` i `possibility_html`; CREATE/READ/UPDATE (`enabled`)/DELETE na `sources`/`scrape_targets`. |
| `ui_heavy` | **częściowo (tak)** | Zadanie samo nie zleca zmian UI, ale dodanie 3 portali zmienia zawartość istniejącego widoku `internal/templates/scraped/list.gohtml` (~4× więcej wierszy, brak informacji o pochodzeniu) oraz agregację w `stats/stats.gohtml`. Analiza UI wykonana, decyzja o zakresie poniżej. |

---

## 1. Stan obecny → stan docelowy, obszar po obszarze

### 1.1 Schemat bazy (`internal/database/schema.sql`)

| | Stan obecny | Stan docelowy |
|---|---|---|
| Tabele | `applications`, `possibilities` | + `sources`, + `scrape_targets`, + `possibility_html` |
| Pojęcie portalu | **nie istnieje** | `sources(id, slug UNIQUE, name, created_at)` |
| URL-e listingów | brak — hardkod w Go | `scrape_targets(id, source_id FK, category TEXT, url TEXT, enabled BOOLEAN, created_at)` |
| Powiązanie oferty ze źródłem | brak | `possibilities.source_id BIGINT REFERENCES sources(id)` + indeks |
| Surowy HTML | brak | `possibility_html(possibility_id BIGINT PK REFERENCES possibilities(id) ON DELETE CASCADE, html TEXT NOT NULL, fetched_at TIMESTAMPTZ NOT NULL DEFAULT now())` |
| Dedup | `url TEXT NOT NULL UNIQUE` | **bez zmian** |
| Indeksy | `possibilities_category_created_at_idx` | + `possibilities_source_id_idx`, + `scrape_targets_source_id_idx`, + `UNIQUE (source_id, url)` na `scrape_targets` |

**Luki i konsekwencje:**

- `possibilities.source_id` **musi być nullable** (bez `NOT NULL`) — istniejące wiersze justjoin.it nie mają źródła. `standards/backend/models.md` („Database Constraints") sugeruje `NOT NULL`, ale nie da się tego pogodzić z istniejącymi danymi bez backfillu. Zgodnie z `migrations.md` („Separate Schema and Data") backfill `source_id = (SELECT id FROM sources WHERE slug='justjoin')` musi być **osobnym krokiem danych**, a ewentualne dociśnięcie do `NOT NULL` — osobną, późniejszą zmianą (poza tym zadaniem).
- `possibility_html` na `possibility_id` jako PK realizuje relację 1:1 i daje darmową idempotencję zapisu (`ON CONFLICT (possibility_id) DO UPDATE`), co jest potrzebne, bo `CreatePossibility` jest upsertem i przy powtórnym scrapie zwróci istniejące `id`.
- `ON DELETE CASCADE` na `possibility_html` — inaczej kasowanie oferty będzie blokowane przez FK. `models.md` („Clear Relationships") wprost wymaga świadomego cascade.
- `scrape_targets.category` to **wolny TEXT bez FK** (kategorie żyją w Go, nie w bazie). To świadomy kompromis — patrz decyzja `category-source-of-truth`. Minimalna ochrona: `CHECK (category <> '')` plus walidacja przy starcie crawlera (nieznany slug → log + pominięcie targetu, zgodnie z `error-handling.md` „Graceful Degradation").
- `UNIQUE (source_id, url)` na `scrape_targets` jest potrzebne, bo model **musi dopuszczać wiele URL-i na parę portal × kategoria** (NFJ: `go` + `golang`, `c` + `c++`), więc klucz nie może obejmować kategorii.
- **Żadne DDL nie może hardkodować `public`.** Guard w `deploy.sh:62` już używa `current_schema()` i to jest wzorzec do powtórzenia.

### 1.2 Warstwa sqlc (`internal/database/queries/`, `generated/`)

| | Stan obecny | Stan docelowy |
|---|---|---|
| Pliki zapytań | `applications.sql`, `possibilities.sql` | + `sources.sql` (nowy) |
| `CreatePossibility` | 9 kolumn, `ON CONFLICT (url) DO UPDATE SET posted_at = COALESCE(...)` | 10 kolumn (+ `source_id`), **`ON CONFLICT (url)` nietknięte** |
| `ExistingPossibilityUrls` | `SELECT url FROM possibilities WHERE url = ANY(@urls::text[])` | **nietknięte** |
| Zapis HTML | brak | `CreatePossibilityHtml :exec` z `ON CONFLICT (possibility_id) DO UPDATE` |
| Odczyt targetów | brak | `ListEnabledScrapeTargets :many` — JOIN `scrape_targets` × `sources`, `WHERE enabled` |
| Transakcje | `WithTx` wygenerowane, **0 użyć** | `WithTx` używane w `persist.go` |

**Luki:**

- `CreatePossibility` przy konflikcie **nie aktualizuje `source_id`** (i nie powinno — `DO UPDATE SET` ma zostać ograniczone do `posted_at`, żeby nie ruszać mechanizmu dedupu). Skutkiem jest: oferta widziana najpierw na portalu A i później na B zachowa `source_id` portalu A. To jest **poprawne** przy przyjętym modelu „dedup po URL, bez dedupu semantycznego" — URL-e absolutne różnych portali nigdy nie kolidują, więc realnie to zdarzenie nie zajdzie. Warunek: URL-e muszą być absolutne (patrz 1.4).
- Analogiczny, **realny** przypadek: ta sama oferta w dwóch targetach tego samego portalu (NFJ `/pl/go` i `/pl/golang`; bulldogjob `role,ai` i `skills,Go`). Wtedy `ON CONFLICT` zostawia `category` z pierwszego wpisu i oferta pojawi się tylko w jednej kategorii. To zachowanie **istnieje już dziś** (justjoin.it nie ma nakładających się kategorii, więc się nie ujawniało) i jest zgodne z „dedup po URL nietknięty". Do odnotowania w specyfikacji jako świadome, nie do naprawiania.
- `ListPossibilitiesByCategory` i `GetPossibility` używają `SELECT *`. Po dodaniu `source_id` sqlc przegeneruje `Possibility` z nowym polem — kompilacja to złapie, więc ryzyko jest zerowe. **Ale** to jest dokładnie powód, dla którego `html` **nie może** trafić do `possibilities`: `SELECT *` ciągnąłby setki KB na wiersz do listy. Decyzja o osobnej tabeli jest tu obowiązkowa, nie preferencyjna (`queries.md`, „Select Only Needed Columns").
- Regeneracja: `make sqlc` (`tech-stack.md` podaje nieistniejący `make sqlc-generate` — do poprawienia przy okazji).

### 1.3 `internal/category`

| | Stan obecny | Stan docelowy (rekomendowany) |
|---|---|---|
| `Category` | `{Slug, Name, ScrapeURL}` | `{Slug, Name}` — `ScrapeURL` usunięte |
| `All` | 5 wpisów z URL-ami justjoin.it | 5 wpisów bez URL-i, **nadal w Go** |
| `scrapeParams` const | `"?orderBy=DESC&sortBy=newest"` | usunięte (parametry są częścią URL-i w `scrape_targets`) |
| `BySlug` | bez zmian | bez zmian |

**Uzasadnienie rekomendacji:** kategoria to **taksonomia UI** (menu, `/scr4p3d/{slug}`, nagłówki), a nie konfiguracja crawlera. Do bazy przenosimy to, o co prosi zadanie — **URL-e listingów** — a nie całą taksonomię. Przy tym wariancie ripple jest zerowy: `templates/parse.go:13`, `handlers/helper.go:80`, `stats_handler.go:44`, `scraped_handler.go:46`, `partials/header.gohtml` używają wyłącznie `Slug` i `Name`. Zmienia się **tylko** `fetcher.go` i `cmd/crawler/main.go`.

**Wariant alternatywny (kategorie z bazy) kosztuje znacznie więcej:** `templates.funcs` to zmienna pakietowa inicjalizowana przy starcie pakietu (`parse.go:12-14`), więc zasilenie jej z bazy wymaga zmiany sygnatury `templates.Parse` i przewleczenia listy przez **5 wywołań** w `cmd/web/main.go:69-97`, a `cmd/web` musiałby czytać `scrape_targets` tylko po to, żeby narysować menu. To narusza `minimal-implementation.md` („No Speculative Abstractions") — kategorii jest 5 i nie zmieniają się.

### 1.4 Fetcher (`internal/crawler/fetcher.go`)

| | Stan obecny | Stan docelowy |
|---|---|---|
| Wejście | `[]category.Category` (konstruktor) | `[]ScrapeTarget` (z bazy) + ekstraktor przypisany do portalu |
| Ładunek wyjściowy | `FetchedOffers{Category string, Urls []string}` | `FetchedOffers{SourceID int64, SourceSlug string, Category string, Urls []string}` |
| Odkrywanie linków | `collectOfferLinks` — twardy selektor `.offer-card` (`:204`) | metoda ekstraktora per portal |
| Mechanika listingu | `scrollAndCollectLinks` — infinite scroll, stop gdy `window.scrollY` przestaje rosnąć (`:168-198`) | jedna z trzech mechanik, wybierana przez ekstraktor |
| Kolejność | bezwarunkowe `slices.Reverse` (`:110`) | per-portal (patrz sekcja 5) |
| Częstotliwość | `frequency = 2h`, `maxFrequencyJitter = 1h` (`:16-17`) | 4h + jitter do 1h |
| Przeglądarka | jedna na cały przebieg wszystkich kategorii (`:64`) | jedna na portal |
| Bufor kanału | `visibleOffers = 100` batchy | bez zmian (patrz sekcja 6) |

**Kluczowa luka — `node.Attribute("href")` (`fetcher.go:208`) zwraca surowy atrybut, nie właściwość DOM.** Dla `<a href="/pl/job/xyz">` da to `/pl/job/xyz`, a nie absolutny URL. Ten string trafia bez zmian do `possibilities.url` i do `ExistingPossibilityUrls`. Konsekwencje, jeśli nie ma normalizacji:

1. `list.gohtml:25` renderuje `<a href="{{ .Url }}">` → relatywny link rozwinie się względem hosta WOK-a i będzie martwy.
2. Dwa portale mogą wyemitować tę samą ścieżkę (`/offer/123/...`) → **kolizja `UNIQUE (url)` między portalami**, czyli dokładnie to ciche zepsucie dedupu, którego zadanie zakazuje.
3. `chromedp.Navigate(url)` z relatywnym URL-em nie zadziała.

**Wymóg**: normalizacja do absolutnego URL-a przy zbieraniu linków — i **ta sama** postać URL-a musi trafić do `ExistingPossibilityUrls` i do `CreatePossibility`. To jedyna twarda zasada chroniąca dedup.
**Do zweryfikowania na produkcji przed implementacją**: czy istniejące wiersze justjoin.it mają URL-e absolutne (`SELECT url FROM possibilities LIMIT 5`). Jeśli są relatywne, jakakolwiek normalizacja **utworzy duplikaty** wszystkich istniejących ofert i musi iść w parze z jednorazowym `UPDATE`.

### 1.5 Transformer (`internal/crawler/transformer.go`)

| | Stan obecny | Stan docelowy |
|---|---|---|
| Ekstrakcja | `scrapeOffer` — monolityczny `chromedp.Run` z selektorami justjoin.it (`:166-225`) | dispatch po `SourceSlug` do ekstraktora portalu |
| Seam pobranie/parsowanie | **brak** | wymagany: `fetchHTML(ctx, url) (string, error)` + `parseOffer(html) (fields, error)` |
| Surowy HTML | nigdy nie pobierany | `chromedp.OuterHTML` / `Evaluate("document.documentElement.outerHTML")` dołożone do istniejącego batcha — **bez dodatkowego żądania sieciowego** |
| Keywords | `stack` z XPath „Tech stack" → `Tech` | tagi/skille każdego portalu → ta sama kolumna `tech`, format `", "` |
| `posted_at` | regex `(?:Published\|Opublikowano)` + składanie ISO **w JS** (`:218-224`) | surowy tekst z DOM, normalizacja **w Go**, jeden parser na format portalu |
| Pauza | `5s`–`12s` (`:20-21`) | `2min`–`3min` |
| `unseen` | 1 zapytanie na batch, przy błędzie zwraca wszystko | **bez zmian** |
| `IsRemote` | `strings.Contains(body, "Remote")` (`:242`) | per portal; anty-wzorzec do niepowielania |

**Luki krytyczne:**

- **Pola wymagane**: `job_title` i `company_name` są `NOT NULL` bez `DEFAULT`, a `scrapeOffer` czyta je przez `chromedp.Text("h1"/"h2")`, które **blokuje do timeoutu** (90 s) jeśli węzła nie ma. Dla portalu bez `h1`/`h2` w oczekiwanym miejscu oznacza to 90 s zmarnowane na ofertę i **zero zapisów** — a więc również **zero zapisanego HTML-a**, mimo że punkt (c) zadania mówi „zapis surowego HTML oferty". Ekstraktory nowych portali nie mogą kopiować tego wzorca. Patrz decyzja `failed-parse-policy`.
- **Ciche `""` przy zepsutym selektorze** (komentarz `:150-152`) — świadomy kompromis przy 1 portalu, przy 4 zamienia się w niewykrywalną degradację. Zgodnie z `error-handling.md` ekstraktory powinny **logować** brak oczekiwanego pola.
- **Czas życia przeglądarki**: `transformBatch` trzyma jeden Chromium na cały batch (`:62-67`). Przy 2-3 min/oferta i 50 ofertach w batchu to **2-2,5 godziny** jednej instancji Chromium. Dziś to 5-10 minut. Wyciek pamięci Chromium staje się realny — do rozważenia recykling przeglądarki co N ofert.

### 1.6 Persist (`internal/crawler/persist.go`)

| | Stan obecny | Stan docelowy |
|---|---|---|
| Interfejs | `OfferStore{ CreatePossibility(ctx, params) (Possibility, error) }` | rozszerzony o transakcję |
| Zapis | jeden `INSERT` | `BEGIN` → `CreatePossibility` (RETURNING id) → `CreatePossibilityHtml` → `COMMIT` |
| Timeout | `3 * time.Second` (`:34`) | podniesiony (rekomendacja: 30 s) |
| Zależność w `main` | `NewOfferPersist(db)` gdzie `db = database.New(pgPool)` | potrzebny `pgPool` (patrz niżej) |

**Luka mechaniczna, którą trzeba rozstrzygnąć w specyfikacji:** `WithTx(tx pgx.Tx)` istnieje w `internal/database/generated/db.go`, ale `DBTX` **nie ma metody `Begin`**. `OfferPersist` trzyma dziś interfejs `OfferStore`, którego `*database.Queries` nie potrafi rozpocząć transakcji. Możliwości:

- **A (rekomendowana)**: `cmd/crawler/main.go` przekazuje do `NewOfferPersist` sam `pgPool` (albo wąski interfejs `txBeginner { Begin(context.Context) (pgx.Tx, error) }` zadeklarowany w pakiecie `crawler`, zgodnie z idiomem „interfejs u konsumenta"), a `persist.go` robi `database.New(tx)` lub `q.WithTx(tx)`. `pgPool` jest już w `main` (`cmd/crawler/main.go:26`).
- **B**: bez transakcji — drugi `INSERT` po `RETURNING id`, z akceptacją, że crash pomiędzy zostawia ofertę bez HTML-a. Łamie `queries.md` („Transactions").

**Timeout**: `3 s` na `INSERT` z 500 KB TEXT-a przy jednoczesnym zapisie z 4 równoległych pipeline'ów jest zbyt ciasny. Uwaga: Postgres kompresuje TEXT automatycznie (TOAST), więc **jawna kompresja w aplikacji jest zbędna** — to odpowiada na pytanie „kompresować?" z analizy kodu.

### 1.7 Konfiguracja (`internal/config/loader.go`, `.env.default`, `docker-compose.yaml`)

| | Stan obecny | Stan docelowy |
|---|---|---|
| Throttling | 4 stałe `const` w 2 plikach | patrz decyzja `throttling-config` |
| Namespace `CRAWLER_` | brak | ewentualnie 4 nowe zmienne |
| Parsowanie `time.Duration` | brak (są tylko `const` `DefaultSessionTTL`, `RequestTimeout`) | wymagałoby `time.ParseDuration` + wartości domyślne |
| Zmienne crawlera w compose | tylko `POSTGRES_*` (`docker-compose.yaml:49-55`) | + throttling (jeśli env), + limity zasobów |

Konflikt standardów: `conventions.md` („Environment Variables: Store configuration in environment variables") vs `minimal-implementation.md` („Build What You Need"). Argument za env: zmiana wartości to `docker compose up -d` (rekreacja kontenera) zamiast pełnego `deploy.sh` z rebuildem obrazu Go — realna różnica przy strojeniu pod bany. Argument przeciw: 4 nowe zmienne + parsowanie + walidacja dla jednoosobowego narzędzia.

### 1.8 Wiązanie (`cmd/crawler/main.go`)

| | Stan obecny | Stan docelowy |
|---|---|---|
| Źródło targetów | `category.All` (`:42`) | `db.ListEnabledScrapeTargets(ctx)` przy starcie |
| Liczba pipeline'ów | 1 (fetcher → transformer → persist) | **N = liczba portali**, po jednym komplecie na portal |
| Mapa ekstraktorów | brak | `map[string]offerScraper` budowana raz w `main()` (idiom `IpRepository` z `cmd/web/main.go:38-58`) |
| `sync.WaitGroup` | 3 jawne `wg.Go` | pętla po źródłach × 3 |
| Zachowanie przy pustej tabeli | n/d | **luka**: crawler bez targetów wystartuje i nic nie zrobi, cicho. Wymagany log ostrzegawczy. |
| Zachowanie przy nieznanym `source.slug` | n/d | **luka**: brak ekstraktora dla sluga z bazy → log + pominięcie (`error-handling.md`, „Graceful Degradation"), nie panic. |

### 1.9 Konsumenci HTTP i szablony

| Plik | Wpływ | Ryzyko |
|---|---|---|
| `internal/handlers/scraped_handler.go` | `ListPossibilitiesByCategory(ctx, cat.Name)` — kluczowanie po `Name` | **Wysokie** — patrz `category-key-mismatch` |
| `internal/handlers/stats_handler.go` | `StatsPossibilityByCategory(ctx, cat.Name)` — agregacja po `tech` | Średnie — rozmycie nazw technologii z 4 portali (zaakceptowane w clarifications) |
| `internal/handlers/helper.go:80` | `requestCategory` → `category.BySlug` | Brak, jeśli `category.All` zostaje w Go |
| `internal/templates/parse.go:13` | `funcs["categories"] = category.All` | Brak, jeśli `category.All` zostaje w Go; **wysokie**, jeśli kategorie idą do bazy |
| `internal/templates/scraped/list.gohtml` | 4 kolumny; ~4× więcej wierszy, brak kolumny źródła | Średnie — patrz `ui-source-column`; **HTML nie może tu trafić** |
| `internal/templates/partials/categorynav.gohtml` | tylko `Slug`/`Name` | Brak |
| `internal/router/routing.go:101-102` | trasy `/st4ts/{category}`, `/scr4p3d/{category}` | Brak nowych tras (chyba że dojdzie filtr źródła) |

### 1.10 Deploy i migracje

| | Stan obecny | Stan docelowy |
|---|---|---|
| Świeża baza | `schema.sql` montowane w `docker-entrypoint-initdb.d` (`docker-compose.yaml:19`) | + nowe tabele |
| Żywa baza | ręcznie nadpisywany heredoc `scripts/deploy.sh:55-79` | patrz decyzja `ddl-delivery` |
| Historia | 3 wersje bloku heredoc, **każda nadpisywała poprzednią** (`cbdb432` → `7181980`) | — |
| Rollback | **brak** | — |
| Seed danych | **nie istnieje** | 21 (lub 22) wierszy `scrape_targets` + 4 wiersze `sources` |
| Schemat | `wok` na VPS (ręcznie), `public` lokalnie | bez zmian, ale DDL wyłącznie przez `current_schema()` |
| `make db-reset` | `DROP SCHEMA public CASCADE` (`Makefile:71`) | **niebezpieczne na VPS** — patrz Ryzyka |

Mechanizm `deploy.sh` narusza **wszystkie cztery** reguły `standards/backend/migrations.md`: brak rollbacku („Reversible"), jeden blok na wszystko („Small and Focused"), brak nazw („Descriptive Names"), nadpisywanie zamiast append-only („Version Control: never modify existing ones after deployment"). To zadanie wnosi największą jak dotąd zmianę schematu (3 tabele + kolumna + 3 indeksy + 21 wierszy seedu), więc jest naturalnym momentem na decyzję.

### 1.11 Testy

| Obszar | Stan obecny | Stan docelowy |
|---|---|---|
| `scrapeOffer` / parsowanie | **0 testów** | testy tabelaryczne per portal na fixture'ach HTML (możliwe dopiero po seamie) |
| `unseen` | `TestUnseen` przez stub | + scenariusz wieloportalowy (te same tytuły, różne URL-e = różne oferty) |
| `optDate` | `TestOptDate` | + parser daty per format portalu |
| `ON CONFLICT (url)` | **0 testów**, przy jawnym zakazie regresji | test integracyjny na realnym Postgresie ze `self-skip` (wzorzec `redis_session_repository_test.go`) |
| Transakcja `possibilities` + `possibility_html` | n/d | ten sam test integracyjny |
| Normalizacja URL-i | n/d | **czysta funkcja → najtańszy i najcenniejszy test w całym zadaniu** |
| Fixture'y HTML | brak konwencji w repo | do wprowadzenia dopiero gdy seam powstanie (`minimal-implementation.md`) |

Idiom do zachowania: stdlib `testing`, tabele `[]struct{name; ...; want}` + `t.Run`, ręczne stuby, nazwy podtestów jako zdania po angielsku. Bez testify.

---

## 2. Analiza cyklu życia danych

### 2.1 Droga danych: URL listingu → wiersz w bazie (model docelowy)

```
[start crawlera]
  db.ListEnabledScrapeTargets(ctx)
    → []ScrapeTarget{ID, SourceID, SourceSlug, Category(slug), URL}
    → grupowanie po SourceSlug
    → dla każdego portalu: osobny pipeline (fetcher/transformer/persist)

[fetcher portalu X]                                    ← Chromium #X, 1 na przebieg
  dla każdego targetu portalu X (sekwencyjnie):
    ekstraktor.CollectOfferURLs(tabCtx, target.URL)     ← infinite scroll | paginacja
    → normalizacja do URL absolutnego                   ← ★ punkt ochrony dedupu
    → (opcjonalne odwrócenie kolejności, per portal)
    → FetchedOffers{SourceID, SourceSlug, Category: slug, Urls}
    → kanał FetchedUrls (bufor 100 batchy)

[transformer portalu X]                                ← Chromium #X', 1 na batch
  unseen(batch) → ExistingPossibilityUrls              ← ★ NIETKNIĘTE
  dla każdego nieznanego URL-a:
    ekstraktor.FetchHTML(tabCtx, url) → rawHTML         ← ★ jedna akcja w istniejącym Run
    ekstraktor.ParseOffer(rawHTML)    → pola            ← ★ testowalny seam
    category.BySlug(batch.Category).Name                ← ★ mapowanie slug → Name
    → CreatePossibilityParams{ ..., SourceID: batch.SourceID, Category: <Name> }
    → kanał Offers (bez bufora), razem z rawHTML
    → pause(2-3 min)

[persist portalu X]
  BEGIN
    CreatePossibility(...) → possibility.ID            ← ★ ON CONFLICT (url) NIETKNIĘTE
    CreatePossibilityHtml(possibility.ID, rawHTML)     ← ON CONFLICT (possibility_id) DO UPDATE
  COMMIT                                                ← timeout 30 s, nie 3 s
```

Punkty oznaczone ★ to miejsca, w których błąd projektowy cicho psuje dedup lub istniejące widoki.

**Gdzie wchodzi `source_id`:** powstaje w `ListEnabledScrapeTargets` (z JOIN-a), jest niesiony przez `FetchedOffers` (nowe pole), wpisywany do `CreatePossibilityParams` w transformerze, zapisywany raz przy `INSERT`. **Nie jest aktualizowany przy `ON CONFLICT`.**

**Gdzie wchodzi `Category`:** `scrape_targets.category` przechowuje **slug**; transformer mapuje go na `Name` przez `category.BySlug` **tuż przed** zbudowaniem `CreatePossibilityParams`. Bez tego mapowania istniejące widoki przestają działać (patrz `category-key-mismatch`).

**Transakcyjność:** `WithTx` istnieje i jest nieużywane. Wymaga dostarczenia `pgx.Tx`, czego `DBTX` nie potrafi — `persist.go` musi dostać `pgPool` albo wąski `txBeginner`. Szczegóły w 1.6.

**Timeout w `persist.go`:** `3 s` → rekomendacja `30 s`. Uzasadnienie: 4 równoległe pipeline'y zapisujące setki KB na wiersz do wspólnej puli. Kompresja aplikacyjna niepotrzebna (TOAST).

**Seed `sources`/`scrape_targets`:**
- *Pierwsze uruchomienie (świeży wolumen)*: `schema.sql` z `docker-entrypoint-initdb.d`. Bez seedu tutaj `make db-reset` i każda nowa maszyna deweloperska dają crawlera, który startuje i **nic nie robi, cicho**.
- *Deploy na istniejącą bazę*: `schema.sql` się **nie wykona**. Seed musi przejść drogą DDL z `deploy.sh` (idempotentnie: `INSERT ... ON CONFLICT (slug) DO NOTHING` dla `sources`, `ON CONFLICT (source_id, url) DO NOTHING` dla `scrape_targets`).
- `migrations.md` („Separate Schema and Data") mówi, żeby dane trzymać osobno od DDL. Patrz decyzja `seed-placement`.

### 2.2 Kompletność CRUD — weryfikacja trójwarstwowa

Weryfikacja wykonana przez `grep` na całym repozytorium (`--include=*.go --include=*.sql --include=*.gohtml`). Wynik dla słowa `source`: **3 trafienia, wszystkie nieistotne** — dwa nagłówki `// source: *.sql` generowane przez sqlc i jeden komentarz „single source of truth" w `category.go:16`. Pojęcie źródła nie istnieje w żadnej warstwie.

#### Encja: `sources` / `scrape_targets` (konfiguracja crawlera)

| Operacja | 1. Backend | 2. Komponent UI | 3. Dostęp użytkownika | Status |
|---|---|---|---|---|
| CREATE | ✅ docelowo: seed w DDL / `psql` | ❌ brak formularza | ❌ brak trasy | ❌ orphan |
| READ | ✅ docelowo: `ListEnabledScrapeTargets` (crawler) | ❌ nic nie wyświetla targetów | ❌ brak trasy | ❌ orphan (odczyt tylko maszynowy) |
| UPDATE (`enabled`) | ⚠️ tylko ręczny `UPDATE` przez `psql` | ❌ brak przełącznika | ❌ brak trasy | ❌ orphan |
| DELETE | ⚠️ tylko `psql` | ❌ | ❌ | ❌ orphan |

**Kompletność (warstwa użytkownika): 0%.** Flaga `enabled` jest w clarifications uzasadniona tym, że „pojedyncze kombinacje można wyłączać **bez rekompilacji**" — ale bez UI wyłączenie wymaga `sudo docker compose exec postgres psql`. To wciąż lepsze niż rekompilacja, więc uzasadnienie się broni; nie jest to jednak „bez dotykania serwera". Wymaga świadomej zgody (`sources-crud-orphan`).

#### Encja: `possibility_html`

| Operacja | 1. Backend | 2. Komponent UI | 3. Dostęp użytkownika | Status |
|---|---|---|---|---|
| CREATE | ✅ docelowo `CreatePossibilityHtml` | n/d (crawler) | n/d | ✅ |
| READ | ❌ **żadne zapytanie nie czyta tej tabeli** | ❌ | ⚠️ pośrednio: `make db-dump` / `make db-backup` (`Makefile:47,60`) | ❌ **orphan: CREATE bez READ** |
| UPDATE | ✅ przez `ON CONFLICT` | n/d | n/d | ✅ |
| DELETE / retencja | ❌ brak polityki, brak `CASCADE` bez jawnej deklaracji | ❌ | ❌ | ❌ |

**Kompletność: 50%.** To jest podręcznikowy przypadek „CREATE bez READ": dane wpadają i nikt ich nie ogląda w aplikacji. Zamierzeniem zadania jest, że odczyt odbywa się **poza aplikacją** (ręczna deduplikacja LLM-em — jawnie poza zakresem), a `make db-dump` daje realną drogę wyjścia. To jest sensowne, ale **musi być świadomą decyzją**, nie skutkiem ubocznym — patrz `possibility-html-orphan`. Wraz z nią idzie polityka retencji: `pg_dump` w `db-backup` uruchamiany przy **każdym** deployu (`deploy.sh:24-29`) zacznie zrzucać gigabajty.

#### Encja: `possibilities.source_id`

| Operacja | 1. Backend | 2. Komponent UI | 3. Dostęp użytkownika | Status |
|---|---|---|---|---|
| CREATE | ✅ docelowo w `CreatePossibility` | n/d | n/d | ✅ |
| READ | ✅ przez `SELECT *` w `ListPossibilitiesByCategory` | ❌ `list.gohtml` nie ma kolumny źródła | ⚠️ trasa istnieje, pole niewidoczne | ❌ **orphan: dane odczytane, ale niepokazane** |

**Kompletność: 50%.** Dane docierają do handlera (`SELECT *`) i są tam odrzucane, bo `possibilityRow` (`scraped_handler.go:28-33`) ich nie mapuje. Patrz `ui-source-column`.

**Łączna kompletność cyklu życia w zakresie zaproponowanym dziś: ~45%.**

### 2.3 Odkrywanie punktów styku (multi-touchpoint)

Wszystkie miejsca, w których „pochodzenie oferty" ma lub mogłoby mieć znaczenie:

| Punkt styku | Dziś | Krytyczność | Objęty zakresem? |
|---|---|---|---|
| Lista ofert `/scr4p3d/{category}` (`list.gohtml`) | 4 kolumny, brak źródła | **Wysoka** — to główny widok, urośnie ~4× | do decyzji |
| Statystyki `/st4ts/{category}` | agregacja po `tech` z 4 portali w jednym worku | Średnia | świadomie poza zakresem (clarifications p. 3) |
| Nawigacja `partials/header.gohtml` | linki per kategoria | Niska | poza zakresem |
| Logi crawlera | `log.Printf("[%s %d/%d] ...", batch.Category, ...)` | **Wysoka** — przy 4 równoległych pipeline'ach log bez nazwy portalu staje się nieczytelny | **powinien wejść** (koszt: 1 pole w formacie) |
| Eksport `make db-dump` | pełny dump | Średnia | poza zakresem (działa) |
| Ręczna deduplikacja LLM-em | poza aplikacją | n/d | jawnie poza zakresem |

**Brakujący punkt styku, którego nikt jeszcze nie wymienił: logi.** Dziś prefiks loga to sama kategoria. Przy 4 pipeline'ach naraz („[Go 3/50] Scraping: ...") nie da się odróżnić, który portal jest crawlowany. To najtańsza zmiana w całym zadaniu (jedno pole w `Printf`) i konieczna, żeby cokolwiek zdiagnozować w produkcji. Traktuję to jako wymóg, nie decyzję.

---

## 3. Ocena wpływu na ścieżkę użytkownika

| Wymiar | Stan obecny | Stan docelowy bez zmian w UI | Stan docelowy z kolumną „Source" | Ocena |
|---|---|---|---|---|
| **Osiągalność** | `/scr4p3d/{slug}` z menu, 1 klik | bez zmian | bez zmian | ✅ 0 |
| **Odkrywalność źródła oferty** | 0/10 (pojęcie nie istnieje) | **1/10** — jedyną wskazówką jest domena w kolumnie URL, na końcu wiersza | **8/10** — dedykowana kolumna, wzorzec standardowy | ❌ / ✅ |
| **Integracja z przepływem** | lista → klik w URL → portal | ⚠️ lista ~4× dłuższa, wizualnie jednorodna, ta sama oferta może wystąpić 2-4 razy z różnych portali (dedup semantyczny poza zakresem) | ⚠️ ta sama długość, ale duplikaty są rozpoznawalne | ⚠️ |
| **Wielu person** | jeden użytkownik (`vision.md`) | n/d | n/d | ✅ |

**Wniosek**: jedyna realna regresja UX to punkt „Odkrywalność". Kolumna URL *technicznie* niesie nazwę portalu, ale jest ostatnią kolumną w tabeli i zawiera pełny, długi adres — to poziom 1-2/10 wg skali (wymaga wcześniejszej wiedzy, żeby wiedzieć, czego szukać). Koszt naprawy jest bardzo niski: pole `Source` w `possibilityRow` (`scraped_handler.go:28`), mapa `id → name` zbudowana raz w handlerze albo JOIN w `ListPossibilitiesByCategory`, jedna kolumna `<th>`/`<td>` w `list.gohtml`. Uwaga: `ListPossibilitiesByCategory` używa `SELECT *`, więc wyciągnięcie **nazwy** źródła wymaga JOIN-a (co zmienia sygnaturę wygenerowanego typu na `ListPossibilitiesByCategoryRow`) albo trzymania małej mapy w handlerze — drugie jest tańsze i nie rusza istniejącego zapytania.

---

## 4. Równoległość per portal: shutdown, RAM, kontener

### 4.1 Graceful shutdown

Dziś łańcuch zamykania jest jednowątkowy i poprawny:
`cancel(ctx)` → `pause()` zwraca `false` (`transformer.go:135-145`) → `fetcher.Fetch` robi `close(of.FetchedUrls)` (`fetcher.go:54`) → `Transform` wychodzi z `range` i robi `close(ot.Offers)` (`transformer.go:49`) → `Persist` wychodzi z `range` → `wg.Wait()`.

Właścicielem każdego kanału jest dokładnie jeden goroutine — dlatego to działa.

| Wariant | Opis | Ryzyko |
|---|---|---|
| **A — N niezależnych pipeline'ów (rekomendowany)** | Po jednym `OfferFetcher`/`OfferTransformer`/`OfferPersist` na portal; każda gałąź ma **własne** kanały `FetchedUrls` i `Offers`; `main()` robi `wg.Go` × 3 w pętli po portalach | Zero zmian w semantyce zamykania — każdy kanał nadal ma jednego właściciela. Koszt: 4× więcej goroutine'ów i 4 pisarzy do puli pg (`pgxpool` domyślnie ma `max_conns = max(4, NumCPU)` — przy 4 pipeline'ach warto to sprawdzić) |
| **B — N fetcherów → wspólny kanał** | Jeden `FetchedUrls` zasilany przez N fetcherów | **`panic: close of closed channel`** — każdy fetcher wywołałby `close`. Wymagałoby dodatkowego `sync.WaitGroup` + osobnego goroutine-zamykacza. Niepotrzebna komplikacja |

Wariant A jest też jedynym, który realizuje wprost decyzję z clarifications: „osobna instancja Chromium i **osobny throttling na portal**" — throttling żyje w `transformBatch`, więc osobny transformer per portal daje go za darmo.

**Nowa luka w wariancie A**: `FetchedUrls` ma bufor 100 batchy (`visibleOffers`, `fetcher.go:15,38`). Przy 5-7 targetach na przebieg i przebiegu co 4-5h bufor zapełni się po ~15-20 przebiegach, czyli po ~3 dniach, jeśli transformer nie nadąża. Wtedy fetcher zablokuje się na wysyłce — ale **`select` z `ctx.Done()` (`fetcher.go:78-82`) sprawia, że shutdown nadal działa**. Deadlocku nie ma; jest ciche zatrzymanie odświeżania listingów. Do udokumentowania.

### 4.2 RAM i kontener

Liczba jednocześnie żywych instancji Chromium:

| Etap | Dziś | Po zmianie (4 portale) |
|---|---|---|
| Fetcher | 1 (żyje na czas przebiegu) | 4 |
| Transformer | 1 (żyje na czas batcha) | 4 |
| **Szczyt jednocześnie** | **2** | **8** |

Przy ~150-400 MB na headless Chromium daje to **1,2-3,2 GB szczytowo** wobec ~300-800 MB dziś.

Konkretne luki w `docker-compose.yaml` (usługa `crawler`, linie 45-63):

- **Brak `mem_limit`/`deploy.resources`** — przy OOM na VPS jądro zabije kontener, a `restart: always` wprowadzi pętlę restartów w trakcie backfillu.
- **Brak `shm_size`** — Docker domyślnie daje `/dev/shm` **64 MB**. Chromium przy 8 instancjach niemal na pewno padnie z „Target closed"/awariami kart. Do wyboru: `shm_size: 1gb` w compose **albo** `chromedp.Flag("disable-dev-shm-usage", true)` w `newBrowser` (`browser.go:20-23`). Druga opcja jest tańsza i nie wymaga zmian w infrastrukturze.
- `init: true` **jest już ustawione** (`docker-compose.yaml:51`) — dobrze, bo Chromium sieroci procesy zygote/crashpad; przy 8 instancjach ich liczba rośnie proporcjonalnie i bez initu byłoby to fatalne.
- `docker/crawler/Dockerfile` — **bez zmian**. Chromium i zależności są już zainstalowane, nowe portale nie wnoszą nowych pakietów systemowych.
- Długość życia jednej instancji rośnie z ~5-10 min do ~2-2,5 h (batch 50 ofert × 2,5 min) — patrz uwaga o recyklingu przeglądarki w 1.5.

---

## 5. Różnice mechaniki listingu między portalami

| Portal | Mechanika | Sortowanie „najnowsze" | Wpływ na `scrollAndCollectLinks` |
|---|---|---|---|
| justjoin.it | infinite scroll, lista wirtualizowana (węzły montowane/odmontowywane) | ✅ `?orderBy=DESC&sortBy=newest` | wzorzec istniejący, bez zmian |
| nofluffjobs.com | nieznana (do ustalenia w Fazie 5/7) | ✅ `?sort=newest` (zweryfikowane) | nieznany |
| bulldogjob.pl | **paginacja, ~50 ofert/stronę** (limit widoczny w weryfikacji URL-i) | ❌ nie znaleziono parametru | **niekompatybilna** — pętla scrolla zakończy się po jednej stronie i zwróci maks. 50 ofert |
| solid.jobs | nieznana | ❌ nie znaleziono parametru | nieznany |

**Wniosek dla kształtu interfejsu ekstraktora — kluczowy dla specyfikacji:**

Projekt „sparametryzujmy selektor `.offer-card`" **nie zadziała**. Bulldogjob wymaga zupełnie innej pętli sterującej (następna strona zamiast scrolla), więc granica abstrakcji musi obejmować **całe odkrywanie linków**, nie sam selektor:

```
type offerScraper interface {
    CollectOfferURLs(ctx context.Context, listingURL string) ([]string, error)
    ScrapeOffer(ctx context.Context, url string) (offerFields, string, error)  // pola + raw HTML
}
```

`scrollAndCollectLinks` (`fetcher.go:168-198`) i `collectOfferLinks` (`:202-212`) schodzą z roli metod `OfferFetcher` do **współdzielonych helperów** pakietu, sparametryzowanych selektorem, i są wywoływane przez te ekstraktory, które faktycznie mają infinite scroll (justjoin.it, prawdopodobnie NFJ). Bulldogjob dostaje własną pętlę paginacyjną. To jest zgodne z `minimal-implementation.md`: abstrakcja jest uzasadniona, bo potrzeba jest **natychmiastowa i konkretna** (4 istniejące portale), ale jej rozmiar musi zostać przy dwóch metodach — bez rejestru pluginów, bez map selektorów sterowanych konfiguracją.

**Nowa luka wynikająca z paginacji**: pętla stron jest **nieograniczona**. `role,architect` i `role,ai` na Bulldogjobie zwracają 50 wyników na stronie, czyli mają ich więcej — bez limitu głębokości pierwszy przebieg może zebrać setki lub tysiące URL-i, które przy 2-3 min/oferta zamienią się w tygodnie crawlowania. Patrz `pagination-depth`.

**Nowa luka wynikająca z braku sortowania**: `slices.Reverse` (`fetcher.go:110`) stosowane jest **bezwarunkowo** do każdego listingu. Komentarz w kodzie (`:106-109`) wprost tłumaczy, że działa to tylko dlatego, że listing jest posortowany newest-first, a `list.gohtml` sortuje po `created_at DESC` — czyli po kolejności wstawiania. Dla Bulldogjoba i solid.jobs kolejność listingu jest nieznana (prawdopodobnie po trafności), więc jej odwrócenie da **kolejność arbitralną**, a widok „najnowsze na górze" przestanie być prawdziwy dla połowy portali. Kolumna `Published` w `list.gohtml:23` pokazuje `posted_at`, więc sortowanie po nim byłoby uczciwsze — ale zmienia istniejące zapytanie i istniejący indeks (`possibilities_category_created_at_idx` nie obsłuży `posted_at`), a `posted_at` bywa NULL. Patrz `listing-ordering`.

---

## 6. Ryzyka

### 6.1 Co może po cichu zepsuć dedup po URL

| Ryzyko | Mechanizm | Waga |
|---|---|---|
| **Relatywne vs absolutne `href`** | `cdp.Node.Attribute("href")` (`fetcher.go:208`) zwraca surowy atrybut. Dla `<a href="/pl/job/x">` da `/pl/job/x`. Dwa portale mogą wyemitować identyczną ścieżkę → kolizja `UNIQUE (url)` **między portalami**, czyli utrata oferty bez śladu w logach (`ON CONFLICT DO UPDATE` zwróci sukces) | **Krytyczna** |
| **Zmiana normalizacji URL-i po fakcie** | Jeśli istniejące wiersze justjoin.it są relatywne, a nowy kod zapisuje absolutne, powstanie komplet duplikatów wszystkich dotychczasowych ofert. Odwrotnie — jeśli są absolutne, a nowy kod nie normalizuje, nowe portale wpadną relatywne | **Krytyczna** — decyduje o tym, czy trzeba jednorazowy `UPDATE` |
| **Parametry śledzące w URL-ach ofert** | Jeśli listing linkuje do `/offer/123?utm_source=list`, ten sam URL z innego targetu może mieć inny parametr → dwa wiersze na jedną ofertę | Wysoka |
| **Przekierowania kanoniczne solid.jobs** | `approved-urls.md`: „portal sam przekierowuje na adres kanoniczny". Jeżeli gdziekolwiek zapiszemy `location.href` po nawigacji zamiast URL-a z listingu, `unseen` (porównujący URL-e z listingu) przestanie trafiać i **każdy przebieg przescrapuje wszystko od nowa** — przy 2-3 min/oferta oznacza to nieskończoną pętlę i szybki ban | **Krytyczna** |
| Zmiana klucza na `UNIQUE (source_id, url)` | Wymagałaby jednoczesnej zmiany `ON CONFLICT` i `ExistingPossibilityUrls`; pominięcie któregokolwiek = ciche pęknięcie | Krytyczna — **objęte zakazem, nie robić** |

**Zasada nadrzędna dla specyfikacji**: string zapisywany do `possibilities.url` musi być **bit w bit** tym samym stringiem, który trafia do `ExistingPossibilityUrls`. Normalizacja (jeśli w ogóle) musi zachodzić **raz**, w fetcherze, przed wysłaniem `FetchedOffers` — nigdy w transformerze ani w persist.

### 6.2 Co może zepsuć istniejące widoki

| Ryzyko | Mechanizm | Waga |
|---|---|---|
| **Zapisanie sluga zamiast `Name` do `possibilities.category`** | `fetcher.go:73` zapisuje `cat.Name`; `scraped_handler.go:55` i `stats_handler.go:53` odpytują po `cat.Name`. `scrape_targets` kluczuje po slugu. Zapis sluga → obie strony rozjeżdżają się i **oba widoki pokazują pustą listę bez żadnego błędu** | **Krytyczna** |
| Nowe kolumny w `possibilities` | `SELECT *` przegeneruje `Possibility`; `scraped_handler.go:64-69` mapuje pola jawnie → kompilator złapie. Ryzyko zerowe | Niska |
| `raw_html` w `possibilities` (odrzucone) | `ListPossibilitiesByCategory` (`SELECT *`) ciągnąłby setki KB × liczba ofert do RAM-u handlera przy każdym żądaniu, przy `handlerTimeout = 5s` (`cmd/web/main.go:109`) | Krytyczna — **rozwiązane** przez osobną tabelę |
| Rozmycie statystyk | `StatsPossibilityByCategory` agreguje `string_to_array(tech, ',')`; 4 portale wniosą „JS"/„JavaScript"/„Javascript" | Średnia — **zaakceptowane** w clarifications p. 3 |
| Usunięcie `category.All` | `templates/parse.go:12-14` to zmienna pakietowa; `funcs` musiałoby być zasilane z bazy, co zmienia sygnaturę `Parse` i 5 wywołań w `cmd/web/main.go` | Wysoka — **unikane** przez rekomendację z 1.3 |

### 6.3 Dryf schematu `wok` / `public`

| Ryzyko | Mechanizm | Waga |
|---|---|---|
| **`make db-reset` na VPS** | `Makefile:71` robi `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`. Na VPS tabele żyją w `wok`, więc to nic nie usunie, a następnie `psql -f schema.sql` trafi (przez `search_path = "$user", public`, gdzie `$user` = `wok`) w schemat `wok` i **padnie na `relation "possibilities" already exists`** albo, przy `IF NOT EXISTS`, zostawi bazę w stanie częściowym | **Krytyczna** — `db-reset` jest dziś niebezpieczny i po dodaniu 3 tabel będzie bardziej |
| Brak `CREATE SCHEMA wok` w repo | Schemat istnieje wyłącznie dzięki ręcznej operacji na VPS. Odtworzenie środowiska od zera jest niemożliwe z samego repo | Wysoka |
| DDL z jawnym `public.` | Tabele wylądowałyby tam, gdzie aplikacja ich nie widzi (komentarz `deploy.sh:48-53` opisuje ten dokładny scenariusz) | Krytyczna — reguła: **wyłącznie `current_schema()`** |
| Rozjazd lokalne/prod | Lokalnie tabele w `public`, na prod w `wok`. `schema.sql` (przez initdb) i heredoc w `deploy.sh` mogą się rozejść, jeśli zmiana trafi tylko do jednego | Wysoka |

**Rekomendacja niezależna od decyzji**: dołożyć do `deploy.sh`, **przed** blokiem DDL, asercję `SELECT current_schema()` z twardym `ON_ERROR_STOP` i jawnym komunikatem, jeśli wynik nie jest oczekiwany. Dziś to niezweryfikowane założenie oparte na ręcznej konfiguracji VPS-a.

### 6.4 Pierwszy backfill 4 portali

Szacunek przy 2,5 min/oferta i ~50 nowych ofert na target:

| Portal | Targetów | Szacowane oferty | Czas jednego pipeline'u |
|---|---|---|---|
| justjoin.it | 5 | już zescrapowane (`unseen` odfiltruje) | ~0 |
| nofluffjobs.com | 7 | ~350 | **~14,5 h** |
| bulldogjob.pl | 5 | ≥ 150 (dwa targety na limicie 50/stronę → przy paginacji **znacznie więcej**) | ≥ 6 h, potencjalnie dni |
| solid.jobs | 5 | ~250 | **~10,5 h** |

Ponieważ pipeline'y są równoległe, całość to `max(...)`, nie suma — czyli **około doby przy optymistycznych założeniach, kilka dni przy pesymistycznych** (Bulldogjob z pełną paginacją).

| Ryzyko | Mechanizm | Waga |
|---|---|---|
| Kolejka rośnie szybciej, niż jest konsumowana | Fetcher wyzwala nowy przebieg co 4-5 h, transformer potrzebuje 10-15 h. `unseen` jest liczone **przy odbiorze batcha**, więc stare batche same się przefiltrowują — to samo-korygujące. Ale bufor 100 batchy zapełni się po ~3 dniach i fetcher zablokuje się na wysyłce (bez deadlocku, `select` z `ctx.Done()` działa) | Średnia — do udokumentowania |
| Ban po stronie portalu | **Brak User-Agenta w całym repo** (grep: 0 trafień) — używany jest domyślny UA headless Chromium, powszechnie wykrywany. Brak proxy, brak obsługi CAPTCHA, brak ciasteczek | Wysoka — zewnętrzna |
| OOM na VPS | 8 instancji Chromium, brak `mem_limit`, `restart: always` → pętla restartów, każdy restart zaczyna backfill od nowa (choć `unseen` chroni przed powtórnym scrapowaniem tego, co już zapisano) | Wysoka |
| `/dev/shm` 64 MB | Awarie kart Chromium objawiające się jako „Target closed", trudne do zdiagnozowania | Wysoka |
| Wzrost backupów | `deploy.sh:24-29` robi pełny `pg_dump` przy **każdym** deployu. Tysiące ofert × setki KB HTML-a | Średnia |

### 6.5 Brak sortowania na 2 portalach

| Ryzyko | Mechanizm | Waga |
|---|---|---|
| Bezwarunkowe `slices.Reverse` | `fetcher.go:110` odwraca każdy listing. Dla listingu posortowanego po trafności daje to kolejność arbitralną → `created_at DESC` w `ListPossibilitiesByCategory` przestaje oznaczać „najnowsze na górze" dla Bulldogjoba i solid.jobs | Średnia (UX, nie utrata danych) |
| Niepełne pokrycie przy paginacji | Bez sortowania po dacie nie ma gwarancji, że pierwsze N stron zawiera najnowsze oferty — limit głębokości paginacji może **systematycznie pomijać** świeże oferty | Wysoka |
| `posted_at` jako alternatywa | Naprawia sortowanie, ale bywa NULL i wymaga zmiany istniejącego zapytania + indeksu | — |

---

## 7. Kwestie wymagające decyzji

> Kwestie już rozstrzygnięte w `clarifications.md` i `approved-urls.md` (model źródeł, osobna tabela HTML, reuse `tech`, równoległość per portal, same URL-e) **nie są tu powtarzane**.

### Krytyczne (do rozstrzygnięcia przed specyfikacją)

**1. `ddl-delivery` — sposób dostarczenia DDL na żywą bazę**
Ta zmiana to 3 tabele + kolumna + 3 indeksy + 21 wierszy seedu — największa dotąd. Heredoc w `deploy.sh` był 3 razy nadpisywany (`cbdb432` → `7181980`) i narusza wszystkie 4 reguły `migrations.md`.
- A: rozszerzyć istniejący heredoc idempotentnie (spójne z repo, nadal łamie standard)
- B: wprowadzić `internal/database/migrations/NNN_opis.sql` + tabela `schema_migrations`, `deploy.sh` aplikuje po kolei
- C: pojedynczy nazwany plik `internal/database/migrations/001_multi_portal_sources.sql`, aplikowany przez `deploy.sh`, w pełni idempotentny, bez tabeli śledzącej
**Rekomendacja: C** — daje „Descriptive Names" i append-only z `migrations.md` przy niemal zerowym koszcie, nie wprowadzając frameworka do jednoosobowego projektu (`minimal-implementation.md`). B to osobne zadanie infrastrukturalne.

**2. `seed-placement` — gdzie żyje seed `sources` + 21 wierszy `scrape_targets`**
`schema.sql` nie wykona się na istniejącej bazie; sam `deploy.sh` zostawia świeżą instalację i `make db-reset` z pustym crawlerem.
- A: w obu miejscach (duplikacja 21 wierszy — naruszenie DRY, ryzyko rozjazdu)
- B: tylko w pliku migracji/`deploy.sh`, `schema.sql` zawiera samo DDL (świeża instalacja startuje bez targetów — trzeba raz uruchomić migrację)
- C: osobny plik `seed.sql` włączany przez oba wejścia
**Rekomendacja: C** — realizuje „Separate Schema and Data" z `migrations.md`, jedno źródło prawdy dla 21 wierszy, brak duplikacji.

**3. `category-source-of-truth` — czy `category.All` znika**
- A: **zostaje w Go**, traci tylko pole `ScrapeURL`; do bazy idą wyłącznie URL-e (`scrape_targets`)
- B: znika, kategorie ładowane z bazy przez obie binarki
**Rekomendacja: A** — zadanie prosi o „źródła (portale) i URL-e w bazie", nie o taksonomię kategorii. Wariant B zmusza `cmd/web` do czytania konfiguracji crawlera, zmienia sygnaturę `templates.Parse` i 5 jej wywołań, i narusza `minimal-implementation.md` przy 5 stałych kategoriach. Wariant A ogranicza zmiany do `fetcher.go` i `cmd/crawler/main.go`.

**4. `category-key-mismatch` — slug w `scrape_targets` vs `Name` w `possibilities.category`**
`possibilities.category` przechowuje `"Go"`, `"Software Architect"`, `"C/C++"`; `approved-urls.md` kluczuje targety slugami. Zapis sluga do `possibilities` **wyzeruje oba istniejące widoki bez błędu**.
- A: `scrape_targets.category` trzyma **slug**, crawler mapuje `category.BySlug(slug).Name` tuż przed `CreatePossibilityParams` (zero zmian w danych, zero ryzyka dla widoków)
- B: migracja `possibilities.category` z `Name` na slug + zmiana obu handlerów na `cat.Slug`
**Rekomendacja: A.** Wariant B jest czystszy docelowo, ale to migracja danych na produkcji przy zerowym pokryciu testami widoków — nieproporcjonalne ryzyko wobec korzyści. Jeśli B, to jako osobne zadanie.

**5. `url-canonicalization` — postać URL-a zapisywanego do `possibilities.url`**
Decyzja nieodwracalna: zmiana kanonizacji później utworzy duplikaty wszystkich istniejących wierszy.
- A: bezwzględne wymuszenie URL-a absolutnego (`url.URL.ResolveReference` względem bazowego adresu portalu) i **nic więcej**
- B: A + usunięcie query stringu i fragmentu
- C: bez zmian (zapisujemy surowy `href`)
**Rekomendacja: A**, warunkowo — **wymaga jednorazowej weryfikacji na produkcji**: `SELECT url FROM possibilities LIMIT 5`. Jeśli istniejące wiersze są relatywne, do specyfikacji dochodzi jednorazowy `UPDATE` doklejający `https://justjoin.it`. B odrzucam domyślnie, bo bez znajomości URL-i ofert nowych portali nie wiadomo, czy query string nie niesie identyfikatora. C odrzucam — łamie linki w UI i grozi kolizją między portalami.

**6. `possibility-html-orphan` — tabela zapisywana i przez nic nieczytana**
Weryfikacja trójwarstwowa: CREATE ✅, READ ❌ (brak zapytania, brak komponentu, brak trasy). Zamierzeniem jest odczyt spoza aplikacji (ręczna deduplikacja LLM-em, poza zakresem), a `make db-dump` realnie działa.
- A: akceptujemy tabelę write-only + **polityka retencji** (np. `DELETE` HTML-a starszego niż N dni) i wyłączenie jej z `db-backup`
- B: akceptujemy write-only bez retencji (backupy i wolumen rosną bez ograniczeń)
- C: dokładamy minimalny odczyt w UI (np. `/scr4p3d/{category}/{id}/html`)
**Rekomendacja: A.** C jest sprzeczne z jawnym zakresem. B naraża na to, że `pg_dump` przy każdym deployu (`deploy.sh:24-29`) stanie się w ciągu miesięcy nieużywalny. Sama retencja to jedno zapytanie i jedna decyzja o liczbie dni.

**7. `sources-crud-orphan` — administracja `scrape_targets` wyłącznie przez psql**
Flaga `enabled` została uzasadniona wyłączaniem kombinacji „bez rekompilacji", ale realnie wymaga `sudo docker compose exec postgres psql`.
- A: akceptujemy psql jako jedyny interfejs administracyjny (`vision.md`: jeden użytkownik, autor)
- B: dokładamy do `deploy.sh` / `Makefile` target pomocniczy (`make target-toggle SLUG=... CATEGORY=...`)
- C: pełny CRUD w UI
**Rekomendacja: A**, ewentualnie B jako 5-liniowy dodatek do `Makefile`. C jest wyraźnie ponad potrzebę dla jednoosobowego narzędzia (`minimal-implementation.md`).

**8. `failed-parse-policy` — co zapisać, gdy parsowanie pól padnie**
Dziś błąd w `scrapeOffer` = brak wiersza (`transformer.go:76-81`), więc również **brak zapisanego HTML-a** — mimo że punkt (c) zadania mówi o zapisie surowego HTML-a, a `job_title`/`company_name` są `NOT NULL`.
- A: bez zmian — nieudana oferta jest pomijana w całości (HTML przepada, 2-3 min zmarnowane)
- B: zapisujemy wiersz z HTML-em i placeholderami w polach wymaganych (np. `job_title = '(unparsed)'`), żeby dało się odtworzyć ofertę z HTML-a później
- C: `job_title`/`company_name` stają się nullable
**Rekomendacja: B.** Cały sens zapisu surowego HTML-a to możliwość późniejszego reparsowania; wariant A gubi dokładnie te przypadki, w których byłoby to najbardziej potrzebne (nowy, jeszcze niedostrojony ekstraktor). C zmienia istniejące ograniczenia bazy bez potrzeby. Warunek dla B: placeholder musi być rozpoznawalny w zapytaniu, żeby dało się takie wiersze odfiltrować.

### Ważne (z domyślną odpowiedzią)

**9. `ui-source-column` — kolumna „Source" w `scraped/list.gohtml`**
Odkrywalność źródła spada do 1/10, a lista rośnie ~4× i będzie zawierać te same oferty z różnych portali (dedup semantyczny poza zakresem).
Opcje: dodać kolumnę | nie dodawać | dodać kolumnę + filtr po źródle.
**Domyślnie: dodać kolumnę** (bez filtra). Koszt: pole w `possibilityRow`, mapa `id → name` w handlerze, jeden `<th>`/`<td>`. Filtr wymagałby nowego zapytania i parametru trasy — poza proporcją.

**10. `throttling-config` — env czy `const`**
`conventions.md` mówi „Environment Variables", `minimal-implementation.md` — „Build What You Need". Za env: zmiana wartości = `docker compose up -d` zamiast pełnego `deploy.sh` z rebuildem obrazu Go, co przy strojeniu pod bany ma znaczenie.
**Domyślnie: `const`, ale w jednym miejscu.** Dziś 4 stałe są rozrzucone po `fetcher.go:16-17` i `transformer.go:20-21`. Zebranie ich w jeden blok z komentarzem to 90% korzyści przy zerowym koszcie; env można dołożyć później, gdy okaże się potrzebne.

**11. `bulldogjob-skills-c` — 22. wiersz `scrape_targets`**
`https://bulldogjob.pl/companies/jobs/s/skills,C` to 6 ofert nieobjętych przez `skills,C++`.
**Domyślnie: dodać.** Analogia do zatwierdzonej decyzji o NoFluffJobs (`/pl/c` + `/pl/c++`) jest bezpośrednia, model jawnie dopuszcza wiele URL-i na parę portal × kategoria, a koszt to jeden wiersz seedu. Wymaga jednak osobnej zgody, bo lista 21 URL-i została zatwierdzona jako zamknięta.

**12. `listing-ordering` — Bulldogjob i solid.jobs bez sortowania**
`slices.Reverse` (`fetcher.go:110`) zakłada listing newest-first; dla 2 portali to założenie jest fałszywe.
Opcje: A — `slices.Reverse` staje się flagą per portal (odwracamy tylko tam, gdzie sortowanie potwierdzone); B — `ListPossibilitiesByCategory` zmienia `ORDER BY` na `posted_at DESC NULLS LAST, created_at DESC, id DESC`; C — akceptujemy kolejność arbitralną dla 2 portali.
**Domyślnie: A.** Najmniejsza zmiana, nie rusza istniejącego zapytania ani indeksu, nie zależy od dostępności `posted_at`. B wymagałby nowego indeksu i psuje się przy NULL-ach.

**13. `pagination-depth` — limit stron przy paginacji Bulldogjoba**
Bez limitu `role,architect` i `role,ai` (oba na limicie 50/stronę) mogą wygenerować setki URL-i na przebieg, a bez sortowania po dacie nie ma gwarancji, że najnowsze są na pierwszych stronach.
**Domyślnie: 3 strony na target na przebieg** (~150 ofert), z logiem, gdy limit zostanie osiągnięty. Do zrewidowania po pierwszym backfillu.

**14. `backfill-cap` — limit ofert na przebieg**
Pierwszy backfill to ~doba przy optymistycznych założeniach, kilka dni przy pesymistycznych; fetcher wyzwala nowy przebieg co 4-5 h.
Opcje: A — limit ofert na przebieg per portal (np. 100); B — bez limitu, akceptujemy wielodniowy backfill.
**Domyślnie: B z udokumentowaniem.** `unseen` liczone przy odbiorze batcha samo koryguje zaległości, bufor 100 batchy daje ~3 dni zapasu, a `select` z `ctx.Done()` (`fetcher.go:78-82`) gwarantuje, że shutdown zadziała nawet przy zapełnionym buforze. Limit dodaje stan, który trzeba by śledzić między przebiegami.

**15. `chromium-resources` — zasoby kontenera przy 8 instancjach Chromium**
Brak `mem_limit` i `shm_size` w `docker-compose.yaml`; domyślne `/dev/shm` = 64 MB praktycznie gwarantuje awarie kart.
**Domyślnie: `chromedp.Flag("disable-dev-shm-usage", true)` w `newBrowser` (`browser.go:20-23`) + `mem_limit: 4g` na usłudze `crawler`.** Flaga jest tańsza niż `shm_size` w compose i działa niezależnie od konfiguracji hosta.

**16. `persist-tx-wiring` — jak `OfferPersist` dostaje transakcję**
`WithTx` istnieje, ale `DBTX` nie ma `Begin`, a `OfferPersist` trzyma interfejs `OfferStore`.
Opcje: A — wąski interfejs `txBeginner { Begin(ctx) (pgx.Tx, error) }` zadeklarowany w pakiecie `crawler`, `pgPool` wstrzykiwany w `main()`; B — bez transakcji, drugi `INSERT` po `RETURNING id`.
**Domyślnie: A** — zgodne z `queries.md` („Transactions") i z idiomem „interfejs u konsumenta" z `conventions.md`; `pgPool` jest już w `cmd/crawler/main.go:26`.

**17. `test-integration-db` — test integracyjny na realnym Postgresie**
Zadanie zabrania regresji w dedupie po URL, a `ON CONFLICT (url)` nie ma dziś **żadnego** testu. `tech-stack.md` twierdzi, że w repo nie ma testów (nieaktualne — jest ich 5).
Opcje: A — test integracyjny ze `self-skip` (wzorzec `redis_session_repository_test.go`, `t.Skipf` gdy baza nieosiągalna); B — tylko testy jednostkowe.
**Domyślnie: A.** To jedyna rzecz, która realnie chroni mechanizm objęty zakazem regresji, a wzorzec self-skip już w repo istnieje i nie psuje `go test ./...` bez bazy.

---

## 8. Rekomendacje

1. **Kolejność wdrożenia**: (d) throttling → (a) schemat + seed + `source_id` → (c) raw HTML + transakcja → (b) ekstraktory, portal po portalu, z justjoin.it jako pierwszym refaktorem „bez zmiany zachowania". Punkt (d) natychmiast obniża ryzyko banów przy dalszych eksperymentach.
2. **Przed napisaniem jakiegokolwiek DDL**: sprawdzić na produkcji `SELECT url FROM possibilities LIMIT 5` (absolutne czy relatywne) i `SELECT DISTINCT category FROM possibilities` (potwierdzić, że to `Name`, nie slug). Obie odpowiedzi zmieniają treść specyfikacji.
3. **Napisać test charakteryzujący dla justjoin.it przed refaktorem (b)** — inaczej „refaktor bez zmiany zachowania" jest niesprawdzalny.
4. **Dodać nazwę portalu do wszystkich logów crawlera** (`transformer.go:74,110,126`, `fetcher.go:72,112`). Przy 4 równoległych pipeline'ach bez tego nie da się nic zdiagnozować. To nie jest decyzja — to wymóg.
5. **Dodać asercję `current_schema()` do `deploy.sh`** przed blokiem DDL. Dziś to niezweryfikowane założenie oparte na ręcznej konfiguracji VPS-a, a wnosimy 3 nowe tabele.
6. **Nie kopiować anty-wzorców z `scrapeOffer`**: `strings.Contains(body, "Remote")`, lokalizacja jako „linia po tytule w `innerText`", ciche `""` przy zepsutym selektorze, składanie dat w JS. Normalizacja dat ma się dziać w Go, po jednym parserze na format portalu.
7. **Utrzymać rozmiar abstrakcji**: dwie metody na ekstraktor, jeden plik `snake_case` na portal, `map[string]offerScraper` budowana raz w `main()`. Bez rejestru pluginów, bez map selektorów sterowanych konfiguracją, bez auto-rejestracji (`minimal-implementation.md`).
8. **Poza zakresem, do osobnego zadania**: prawdziwy framework migracji (`migrations.md`), `CREATE SCHEMA wok` w kontroli wersji, naprawa `make db-reset` pod schemat `wok`, normalizacja nazw technologii w `tech`, User-Agent i obsługa anty-bot. Zaktualizować `tech-stack.md` (nieistniejący `make sqlc-generate`, „no migration framework needed", „no test files present").

---

## 9. Ocena ryzyka

| Wymiar | Ocena | Uzasadnienie |
|---|---|---|
| **Złożoność** | Wysoka | 4 podzadania o różnym profilu, ~700 linii Go do zmiany, 4 nowe pliki ekstraktorów, 3 nowe tabele, zmiana kształtu wiązania w `main()` z 1 na N pipeline'ów |
| **Integracja** | Wysoka | 7 unikalnych plików-konsumentów w dwóch binarkach; największa niewiadoma leży **poza kodem** — DOM, model paginacji i postawa anty-botowa 3 nieznanych portali; brak sortowania na 2 z nich |
| **Regresja** | **Bardzo wysoka** | Dedup po URL jest objęty zakazem zmian i ma **zero testów**. Dwa niezależne mechanizmy cichego zepsucia: postać URL-a (absolutny vs relatywny) i klucz kategorii (`Name` vs slug) — oba dają puste lub zduplikowane widoki bez jednego błędu w logach |
| **Operacyjne** | Wysoka | Zmiana schematu produkcji bez frameworka migracji i bez rollbacku; schemat `wok` poza kontrolą wersji; `make db-reset` niebezpieczny; 8 instancji Chromium bez limitów pamięci i z `/dev/shm` 64 MB; wielodniowy pierwszy backfill przy braku User-Agenta |
| **Typów Go** | Niska | sqlc z jawnymi listami kolumn + literały z nazwanymi polami; kompilator złapie każde pominięte miejsce. Zero mapowań `SELECT *` na ręczne struktury |
