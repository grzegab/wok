# Raport Analizy Kodu

**Data**: 2026-09-04
**Zadanie**: Rozszerzenie crawlera o obsługę wielu portali z ofertami pracy (NoFluffJobs, Bulldogjob, solid.jobs)
**Opis**: Dodać do crawlera obsługę nowych portali z ofertami pracy: NoFluffJobs, Bulldogjob, solid.jobs. Dziś crawler obsługuje wyłącznie justjoin.it. Zakres: (a) przenieść definicje źródeł/URL-i do bazy danych, (b) wprowadzić strategię ekstrakcji per-portal, (c) zapisywać surowy HTML oferty + słowa kluczowe + datę publikacji, (d) zmienić throttling na 2-3 min na ofertę i 4-5h na listę ofert z randomizacją. POZA ZAKRESEM: deduplikacja semantyczna między portalami (robiona ręcznie przez użytkownika z pomocą LLM); istniejąca deduplikacja po URL ma zostać nietknięta. Użytkownik musi ręcznie potwierdzić URL-e portali przed implementacją.
**Analiza**: skill codebase-analyzer (3 agenty Explore: Code Analysis + File Discovery, Context Discovery, Pattern Mining)

---

## Podsumowanie

Crawler to czysty, 3-etapowy pipeline goroutine'ów (fetch → transform → persist) spięty dwoma kanałami w `cmd/crawler/main.go`, gdzie **cała wiedza o portalu justjoin.it jest zaszyta na stałe** w trzech miejscach: URL-e w `internal/category/category.go`, selektor `.offer-card` w `fetcher.go` i komplet selektorów/XPath/regex w funkcji `scrapeOffer` w `transformer.go`. Nie istnieje żadne pojęcie "źródła"/"portalu" — ani w kodzie (`Category` miesza "temat" z "witryną"), ani w bazie (tabela `possibilities` nie ma kolumny źródła), ani w konfiguracji (wszystkie parametry throttlingu to stałe `const` kompilowane do binarki).

Największy koszt to punkt (b) — wprowadzenie warstwy strategii per-portal wymaga rozcięcia monolitycznego `scrapeOffer` na seam "pobierz stronę" / "sparsuj pola", co dziś nie istnieje. Punkty (a) i (c) są umiarkowane, ale obciążone **krytycznym ryzykiem infrastrukturalnym**: projekt nie ma frameworka migracji — zmiany schematu na działającej bazie aplikuje się przez ręczną edycję bloku heredoc w `scripts/deploy.sh`, a aplikacja na VPS działa w schemacie `wok`, który **nie jest tworzony nigdzie w repozytorium**. Punkt (d) jest trywialny mechanicznie (istniejąca randomizacja `rand.N` to dokładnie to, o co prosi zadanie), ale ma poważną konsekwencję przepustowościową.

---

## Zidentyfikowane pliki

### Pliki główne

**`internal/justjoin/transformer.go`** (266 linii)
- Etap 2 pipeline'u: filtruje znane URL-e (`unseen`), otwiera przeglądarkę per batch, scrapuje każdą ofertę (`scrapeOffer`), emituje `database.CreatePossibilityParams` na kanał `Offers`.
- **Epicentrum zmiany.** Zawiera 100% selektorów specyficznych dla justjoin.it (h1/h2, XPath "Job description", XPath "Tech stack", regex `(?:Published|Opublikowano)`), monolityczny `chromedp.Run` bez seamu do parsowania, stałe throttlingu (`minOfferPause`, `maxOfferPause`, `offerTimeout`) oraz jedyne miejsce, w którym powstaje literał `CreatePossibilityParams{...}` — czyli miejsce, gdzie trzeba dodać `Source`, `RawHtml`, `Keywords`.

**`internal/justjoin/fetcher.go`** (213 linii)
- Etap 1: pętla przebiegów, przeglądarka per przebieg, zakładka per listing, scroll + zbieranie linków.
- Zawiera stałe `frequency` (2h), `maxFrequencyJitter` (1h) — bezpośredni cel punktu (d). Zawiera selektor `.offer-card` (linia 204) oraz założenie o wirtualizowanym infinite-scrollu i sortowaniu newest-first (`slices.Reverse`, linia 110) — wszystko specyficzne dla justjoin.it, wszystko musi trafić za interfejs strategii.

**`internal/category/category.go`** (35 linii)
- Płaska lista `All []Category{Slug, Name, ScrapeURL}` — **jedyny rejestr źródeł w aplikacji**, konsumowany zarówno przez crawlera, jak i przez warstwę web (menu, handlery, statystyki).
- Cel punktu (a). Struktura konflatuje "kategorię" (Go/PHP) z "portalem" (domena w URL-u) — brak dyskryminatora portalu.

**`internal/database/schema.sql`** (36 linii)
- Definicja tabel `applications` i `possibilities`. Brak jakiejkolwiek tabeli/kolumny źródła. `url TEXT NOT NULL UNIQUE` to cały mechanizm deduplikacji.
- Cele punktów (a) i (c): nowa tabela źródeł + kolumny `raw_html`, `keywords`.

**`internal/database/queries/possibilities.sql`** (~30 linii)
- Zawiera `ExistingPossibilityUrls` i `CreatePossibility` z `ON CONFLICT (url) DO UPDATE` — **mechanizm deduplikacji objęty zakazem zmian**. Wstawianie nowych kolumn do `CreatePossibility` jest bezpieczne; zmiana klucza konfliktu — nie.

**`scripts/deploy.sh`** (~85 linii)
- Blok heredoc "Applying schema updates" — **jedyna droga zastosowania DDL na działającej bazie produkcyjnej**. Każda nowa kolumna/tabela musi tam trafić jako idempotentny DDL.

**`cmd/crawler/main.go`** (69 linii)
- Punkt spięcia pipeline'u i jedyne miejsce wiring-u (`crawler.NewOfferFetcher(category.All)`). Tu wyląduje mapa `portal → strategia` i ładowanie źródeł z bazy.

### Pliki powiązane

**`internal/justjoin/browser.go`** (55 linii)
- `newBrowser` / `newTab` — helpery chromedp współdzielone przez fetcher i transformer. Portal-agnostyczne, do ponownego użycia bez zmian. Miejsce ewentualnego dodania User-Agenta (dziś nie ustawia się żadnego).

**`internal/justjoin/persist.go`** (41 linii)
- Etap 3, interfejs `OfferStore` z jedną metodą. Timeout `3 * time.Second` na insert — **wymaga rewizji** przy zapisie surowego HTML-a (setki KB).

**`internal/database/generated/*`** (models.go, querier.go, possibilities.sql.go, db.go)
- Kod z `sqlc`. Regeneracja: `make sqlc` (uwaga: `tech-stack.md` podaje nieistniejący target `make sqlc-generate` — dokumentacja jest nieaktualna).

**`internal/config/loader.go`** (62 linii)
- Płaskie `os.Getenv`, brak namespace'u `CRAWLER_`. Wzorzec do rozszerzenia, jeśli throttling ma być konfigurowalny.

**`internal/handlers/scraped_handler.go`, `stats_handler.go`, `helper.go`; `internal/templates/parse.go`, `internal/templates/scraped/list.gohtml`, `internal/templates/partials/categorynav.gohtml`; `internal/router/routing.go`**
- Konsumenci `category.All` / `category.BySlug` oraz tabeli `possibilities`. Nie łamią się przy dodaniu kolumn (sqlc + literały z nazwanymi polami = bezpieczeństwo w czasie kompilacji), ale **wszyscy** wymagają przejrzenia przy przeniesieniu źródeł do bazy.

**`internal/repository/redis_ip_repository.go`, `in_memory_ip_repository.go`, `redis_session_repository.go`, `in_memory_session_repository.go`**
- Nie są modyfikowane, ale stanowią **najlepszy wzorzec w repo** dla warstwy strategii per-portal (interfejs u konsumenta + jeden plik na implementację + `New<X>(...)` + wiring w `main()`).

**`internal/justjoin/transformer_test.go`** (73 linii)
- Jedyny test w pakiecie crawler. Pokrywa wyłącznie `optDate` i `unseen` (przez stub `stubKnownOffers`). `scrapeOffer` jest **nietestowany**.

---

## Obecna funkcjonalność

### Przepływ wykonania

```
cmd/crawler/main.go:20 main()
 ├─ config.Load()                       → tylko conf.PostgresDSN jest używany
 ├─ postgres.NewClient(dsn, false)      → pgxpool.Pool
 ├─ db := database.New(pgPool)          → sqlc Queries
 │
 ├─ cr := crawler.NewOfferFetcher(category.All)     [goroutine]
 │    pętla: fetchAll(ctx) → pause(frequency + rand.N(maxFrequencyJitter)) → repeat
 │      fetchAll: newBrowser(ctx)  ← JEDNA przeglądarka na przebieg
 │        dla każdej category.Category:
 │          getUrls(cat.ScrapeURL) → scrapeOnce (3 próby, backoff 2s·2^n)
 │            → scrollAndCollectLinks (scroll co 1s, re-query ".offer-card")
 │            → slices.Reverse (najstarsze pierwsze)
 │          → FetchedOffers{Category, Urls} na kanał FetchedUrls (bufor 100)
 │
 ├─ ot := crawler.NewOfferTransformer(db)           [goroutine]
 │    for batch := range FetchedUrls → transformBatch
 │      unseen(): 1 zapytanie ExistingPossibilityUrls na batch
 │      newBrowser(ctx)  ← JEDNA przeglądarka na batch
 │      dla każdego url: scrapeOffer(url, category) → CreatePossibilityParams
 │        → kanał Offers (bez bufora)
 │        → pause(minOfferPause + rand.N(max-min))  = 5–12 s
 │
 └─ op := crawler.NewOfferPersist(db)               [goroutine]
      for c := range Offers → CreatePossibility(ctx, c)  (timeout 3 s, upsert po url)

Shutdown: SIGINT/SIGTERM → cancel(ctx) → pause() zwraca false →
  fetcher zamyka FetchedUrls → transformer dopina i zamyka Offers → persist dopina → wg.Wait()
```

### Kluczowe komponenty

- **`category.Category{Slug, Name, ScrapeURL}`**: jedyne dzisiejsze pojęcie "źródła"; jeden wiersz = jeden URL listingu justjoin.it, z zaszytym `scrapeParams = "?orderBy=DESC&sortBy=newest"`.
- **`FetchedOffers{Category string, Urls []string}`**: ładunek między etapem 1 a 2. **Brak identyfikatora portalu** — do przewleczenia.
- **`database.CreatePossibilityParams`**: ładunek między etapem 2 a 3 (= struktura insertu z sqlc). Pola: `Category, Url, CompanyName, JobTitle, Location, IsRemote, Tech, Description, PostedAt`. Brak `Source`, `RawHtml`, `Keywords`.
- **`knownOfferStore` / `OfferStore`**: wąskie interfejsy zdefiniowane u konsumenta — jedyne punkty abstrakcji, jakie dziś istnieją, i naturalne wzorce dla nowej warstwy strategii.
- **`newBrowser` / `newTab`**: lifecycle chromedp. `chromedp.NoSandbox`, pusty batch dla wymuszenia startu Chromium.
- **`pause(ctx, d)`**: współdzielony, anulowalny sleep (`time.NewTimer` + `select` na `ctx.Done()`).
- **`scrapeOffer`**: jedno wywołanie `chromedp.Run` robiące nawigację i całą ekstrakcję naraz — brak seamu "pobranie" vs "parsowanie".
- **`optDate` / `optText`**: koercja do `pgtype.*`. `optText` jest portal-agnostyczne i nadaje się do ponownego użycia; `optDate` zakłada, że data przyszła już jako ISO `YYYY-MM-DD` (składana ręcznie w JS-ie!).

### Mechanizm przeglądarki

Headless Chromium przez **`chromedp`** (`github.com/chromedp/chromedp`), nie `net/http`. Potwierdzone w `docker/crawler/Dockerfile` (`apk add chromium`). Przeglądarka startuje raz na przebieg (fetcher) i raz na batch (transformer), zakładka per jednostka pracy z `context.WithTimeout`.

**Brak jakiegokolwiek User-Agenta** (grep po `UserAgent`/`User-Agent` w całym repo: 0 trafień) — używany jest domyślny UA Chromium. Brak rotacji proxy, obsługi CAPTCHA i persystencji ciasteczek.

**Dostępność surowego HTML-a**: kod nigdy nie pobiera `outerHTML`. Ale ponieważ chromedp ma już w pełni wyrenderowaną stronę w zakładce, wystarczy dodać jedną akcję (`chromedp.OuterHTML` lub `chromedp.Evaluate("document.documentElement.outerHTML", &html)`) do istniejącego batcha w `scrapeOffer` — **bez dodatkowego żądania sieciowego**. Koszt strukturalny niski; koszt schematu i przechowywania — realny.

### Przepływ danych

`ScrapeURL (hardcoded)` → listing DOM → `[]string` URL-i → filtr `unseen()` po bazie → strona oferty DOM → wyciągnięte fragmenty tekstu → `CreatePossibilityParams` → upsert po `url` → tabela `possibilities` → handlery web (`/scr4p3d/{category}`, `/st4ts/{category}`).

---

## Zależności

### Importy (od czego zależy crawler)

- `github.com/chromedp/chromedp` + `cdproto/cdp`: sterowanie headless Chromium, cała ekstrakcja.
- `github.com/jackc/pgx/v5` + `pgtype`: pula połączeń i typy nullable.
- `internal/database/generated` (sqlc): `Queries`, `CreatePossibilityParams`, `Querier`.
- `internal/category`: rejestr źródeł.
- `internal/config`, `internal/postgres`: bootstrap.
- `math/rand/v2` (`rand.N`), `time`, `slices`, `strings`, `log` — stdlib.

### Konsumenci (co zależy od zmienianych elementów)

**Konsumenci `category.All` / `category.BySlug`:**
- **`cmd/crawler/main.go:42`**: `crawler.NewOfferFetcher(category.All)`.
- **`internal/templates/parse.go:13`**: budowa menu.
- **`internal/handlers/helper.go:79`**: `requestCategory` — rozwiązanie slug-a z URL-a.
- **`internal/handlers/stats_handler.go:35`**, **`internal/handlers/scraped_handler.go:37`**: walidacja kategorii z requestu.

**Konsumenci tabeli `possibilities`:**
- **`internal/handlers/scraped_handler.go`**: `ListPossibilitiesByCategory` → projekcja na `possibilityRow{Url, JobTitle, Tech, Published}`. Brak filtra po źródle.
- **`internal/handlers/stats_handler.go`**: `StatsPossibilityByCategory` (agregacja `tech`) — nieczuły na nowe kolumny.
- **`internal/templates/scraped/list.gohtml`**: kolumny Title / Published / Technologies / URL.
- **`internal/justjoin/persist.go`, `transformer.go`**: ścieżka zapisu.

**Liczba konsumentów**: 4 pliki dla `category`, 4 pliki dla `possibilities` (7 unikalnych).
**Zasięg wpływu**: **Średni-Wysoki** — zmiana dotyka obu binarek (`cmd/crawler` i `cmd/web`), warstwy szablonów oraz schematu bazy produkcyjnej. Nigdzie nie ma mapowania `SELECT *` na ręczne struktury, więc dodanie kolumn jest bezpieczne w czasie kompilacji — ryzyko leży w migracji i w wiring-u, nie w typach.

---

## Pokrycie testami

### Pliki testowe

- **`internal/justjoin/transformer_test.go`**: `TestOptDate` (tabelaryczny), `TestUnseen` (przez stub `stubKnownOffers`). Nie dotyka ani chromedp, ani prawdziwego SQL-a.
- **`internal/repository/in_memory_session_repository_test.go`**, **`redis_session_repository_test.go`**: repozytoria sesji; ten drugi robi `t.Skipf` gdy Redis nieosiągalny.
- **`internal/router/routing_test.go`**, **`auth_middleware_test.go`**: routing i middleware auth.

### Ocena pokrycia

- **Liczba plików testowych**: 5 w całym repo; **1** w pakiecie `crawler`.
- **Luki**:
  - `scrapeOffer` — **zero testów**; nie da się go dziś przetestować jednostkowo, bo ekstrakcja jest wpleciona w `chromedp.Run`.
  - `fetcher.go` w całości — zero testów.
  - **Zero testów uderzających w prawdziwego Postgresa** — `ON CONFLICT (url) DO UPDATE`, czyli najbardziej krytyczne dla bezpieczeństwa zachowanie w tym zadaniu (objęte zakazem regresji), nie ma żadnego testu.
  - Brak jakiejkolwiek konwencji fixture'ów HTML w repo.

### Idiom testowy do naśladowania

Wyłącznie stdlib `testing`, tabele `[]struct{name; ...; want}` + `t.Run`, ręcznie pisane stuby implementujące wąski interfejs (bez testify, bez frameworka mockującego). Nazwy podtestów to zdania po angielsku (`"keeps the order of the new offers"`), nie forma `shouldReturnXWhenY`.

---

## Wzorce kodowania

### Konwencje nazewnicze

- **Pakiety**: lowercase, jednowyrazowe, zgodne z katalogiem (`crawler`, `category`, `repository`).
- **Pliki**: `snake_case.go`; jeden plik na konkretną implementację, z prefiksem backendu — `redis_ip_repository.go`, `in_memory_ip_repository.go`.
- **Typy**: `<Backend><Rzecz>` PascalCase — `RedisIpRepository`, `InMemorySessionRepository`.
- **Konstruktory**: `New<Backend><Rzecz>(deps...) *<Type>` — zawsze zwykła funkcja zwracająca wskaźnik, bez wzorca Options, bez buildera.
- **Interfejsy**: małe, deklarowane **u konsumenta**, nie w pakiecie implementacji (`router.IpRepository`, `crawler.OfferStore`, `crawler.knownOfferStore`, `handlers.PossibilityStore`).

### Wzorce architektoniczne

- **Styl**: funkcyjny/proceduralny z lekkimi strukturami; brak dziedziczenia, brak frameworka DI.
- **Wiring**: jednorazowy, jawny, w `main()` — `var iface Interface; if cond { iface = NewA() } else { iface = NewB() }` (`cmd/web/main.go:38-58`). Bez fabryk, bez rejestrów, bez refleksji.
- **Konkurencja**: kanały jako złącza pipeline'u, każdy etap eksponuje swój output jako publiczne pole-kanał; `sync.WaitGroup.Go` (Go 1.24+).
- **Błędy**: `fmt.Errorf("...: %w", err)` dla błędów z zależności zewnętrznych; w pętlach goroutine'ów — logowanie i kontynuacja (log-and-continue by design).
- **Logowanie**: wyłącznie stdlib `log`, bez poziomów i bez logowania strukturalnego.
- **Komentarze**: obszerne komentarze "dlaczego" tłumaczące nieoczywiste kompromisy (wirtualizacja listingu, kolejność wstawiania, dlaczego `Evaluate` zamiast `Text`) — mocna, konsekwentna konwencja, którą nowy kod musi utrzymać.

---

## Ocena złożoności

| Czynnik | Wartość | Poziom |
|---------|---------|--------|
| Rozmiar plików | 5 plików głównych, ~700 linii Go do modyfikacji + schema + deploy.sh | Wysoki |
| Liczba plików | 9+ zmienianych, 4 nowe (po jednym na portal) | Wysoki |
| Zależności | ~8 importów w pakiecie crawler | Średni |
| Konsumenci | 7 unikalnych plików (`category` + `possibilities`) | Wysoki |
| Pokrycie testami | 1 plik testowy w crawlerze, `scrapeOffer` nietestowany, 0 testów DB | Wysoki (ryzyko) |

### Ogólnie: Złożone (Complex)

Cztery podzadania o bardzo różnym profilu, spięte wspólnym wymogiem zmiany schematu produkcyjnej bazy bez frameworka migracji. Największa nieznana leży **poza kodem** — struktura DOM, model paginacji i postawa anty-botowa trzech nowych portali. Punkt (d) jest trywialny, ale wpływa na wykonalność całości (patrz Zagrożenia).

### Złożoność per podzadanie

| Podzadanie | Poziom | Uzasadnienie |
|---|---|---|
| (a) Źródła/URL-e do bazy | **Umiarkowane** | `category.All` ma 4 niezależne miejsca wywołania w dwóch binarkach; potrzebna nowa tabela + zapytania sqlc + cache w pamięci przy starcie (żeby handlery nie chodziły do bazy per request) + strategia seed-u; trzeba zachować semantykę `BySlug`. |
| (b) Strategia per-portal | **Złożone** | Największa zmiana. Interfejs + 4 implementacje, każda reverse-engineerowana pod inny DOM; `fetcher.go` i `transformer.go` stają się dispatcherami; `FetchedOffers` i `CreatePossibilityParams` muszą nieść identyfikator portalu; wymaga rozcięcia `scrapeOffer` na seam pobranie/parsowanie. |
| (c) raw HTML + keywords + data | **Umiarkowane** | Raw HTML strukturalnie tani (jedna akcja w istniejącym `chromedp.Run`), ale wymaga rewizji timeoutu 3 s w `persist.go`, świadomości rozmiaru dumpów i decyzji o kompresji. `posted_at` istnieje, ale parser jest justjoin-specyficzny (wiąże się z (b)). **`keywords` to całkowicie nowe pojęcie — brak pola, brak logiki, brak definicji.** |
| (d) Throttling 2-3 min / 4-5 h | **Proste** | Mechanizm randomizacji (`rand.N`) jest już dokładnie taki, jak wymagany; zmieniają się tylko granice. Uczynienie ich konfigurowalnymi to prosty dopisek w `config.Config` + `time.ParseDuration`. |

---

## Kluczowe ustalenia

### Mocne strony

- **Czysta architektura pipeline'u**: trzy etapy, dwa kanały, jawny wiring w `main()`, poprawny graceful shutdown przez `cancel(ctx)` propagowany do `pause()`. Nowe portale nie wymagają zmiany kształtu pipeline'u.
- **Istnieje idealny wzorzec do przeszczepienia**: idiom `IpRepository`/`SessionRepository` (interfejs u konsumenta + jeden plik na implementację + `New<X>` + wybór w `main()`) mapuje się 1:1 na strategię per-portal.
- **Bezpieczeństwo typów przy zmianie schematu**: sqlc z jawnymi listami kolumn i literały ze nazwanymi polami sprawiają, że dodanie kolumn nie skompiluje się, dopóki wszystkie miejsca wstawiania nie zostaną zaktualizowane. Zero mapowań `SELECT *`.
- **Randomizacja throttlingu już istnieje** w dokładnie wymaganej postaci (`rand.N` uniform w przedziale) — punkt (d) to zmiana wartości, nie mechanizmu.
- **Surowy HTML jest "na wyciągnięcie ręki"** — strona jest już wyrenderowana w zakładce chromedp, nie trzeba drugiego pobrania.
- **Silna kultura komentarzy "dlaczego"** ułatwiająca zrozumienie nieoczywistych decyzji (wirtualizacja, kolejność, konserwatywne zachowanie `unseen` przy błędzie).

### Zagrożenia

- **Brak frameworka migracji (KRYTYCZNE)**. `schema.sql` uruchamia się wyłącznie na świeżym wolumenie Postgresa. Zmiany na działającej bazie idą przez ręcznie edytowany blok heredoc w `scripts/deploy.sh`, w którym **każda kolejna zmiana nadpisywała poprzednią** (commit `cbdb432` dodał fix typu `posted_at` + rename indeksu; `7181980` cofnął zmianę indeksu w tym samym bloku). Brak nazwanych, append-only migracji, brak rollbacku, brak audytu poza `git blame` na jednym pliku. To narusza **wszystkie** wymagania z `standards/backend/migrations.md` (Reversible, Small and Focused, Descriptive Names, Version Control).
- **Schemat `wok` nie istnieje w repozytorium (KRYTYCZNE)**. `docker-compose.yaml` tworzy rolę i bazę o nazwie `wok`, ale **żaden plik nie wykonuje `CREATE SCHEMA wok`**; `make db-reset` wręcz robi `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`. `postgresDSN()` nie ustawia `search_path`. Komentarz w `deploy.sh` zakłada, że rola `wok` rozwiąże się do schematu `wok` przez domyślne `search_path = "$user", public` — co działa tylko dlatego, że schemat utworzono ręcznie na VPS-ie, poza kontrolą wersji. To potwierdza wpis w pamięci projektu ("app tables must live in the `wok` schema"). Każdy nowy DDL **nie może** hardkodować `public` — musi używać `current_schema()`, tak jak istniejący guard.
- **Przepustowość przy nowym throttlingu (WYSOKIE)**. Przejście z 5-12 s na 2-3 min na ofertę to ~20-krotne spowolnienie. Przy 5 kategoriach × 4 portalach jeden przebieg z setką nowych ofert na portal trwa **kilkadziesiąt godzin** — dużo dłużej niż interwał 4-5 h między przebiegami. Pipeline jest sekwencyjny (jeden transformer, jedna przeglądarka na batch), więc kolejki będą rosły w nieskończoność. **Wymaga decyzji projektowej**, zanim (d) zostanie wdrożone: limit ofert na przebieg, priorytetyzacja, równoległość per portal, albo rozdzielenie harmonogramu fetch od transform.
- **Zero testów dotykających prawdziwej bazy**, przy czym zadanie explicite zakazuje regresji w deduplikacji po URL. `ON CONFLICT (url) DO UPDATE` nie ma dziś żadnego zabezpieczenia.
- **Brak sygnału o zepsutym selektorze**. `chromedp.Evaluate` celowo zwraca `""` zamiast blokować (komentarz w `transformer.go:150-152`), przez co "portal zmienił layout" jest nieodróżnialne od "pole faktycznie nieobecne". Przy czterech portalach to zamienia się z drobnostki w realny problem operacyjny.
- **Brak User-Agenta i jakiejkolwiek obsługi anty-bot**. Nowe portale mogą być mniej tolerancyjne niż justjoin.it. Realne, ale zewnętrzne ryzyko.
- **Rozmiar `raw_html`**. Setki KB na ofertę × tysiące ofert. Timeout 3 s w `persist.go` jest ciasny, `pg_dump`-owe backupy urosną materialnie, brak w repo jakiegokolwiek precedensu przechowywania dużych blobów.
- **Konflikt standardów wymagający decyzji użytkownika**: `standards/backend/models.md` ("Database Constraints", "Index Foreign Keys", "Clear Relationships") sugeruje pełną tabelę `sources` z FK, ale ten sam dokument mówi "Practical Normalization", a `minimal-implementation.md` — "No Speculative Abstractions". Wybór między denormalizowaną kolumną `source TEXT` a pełną tabelą źródeł z konfiguracją URL-i **musi zostać potwierdzony przez użytkownika** (punkt (a) sugeruje tabelę, bo URL-e mają iść do bazy).
- **`keywords` jest niezdefiniowane**. Nie wiadomo, czy to pole scrapowane z portalu, wyprowadzone z `tech`/`description`, czy generowane. Wymaga decyzji przed implementacją.

### Możliwości

- **Rozcięcie `scrapeOffer` odblokowuje testowalność**. Jeśli strategia rozdzieli "pobierz raw HTML" od "sparsuj pola z HTML-a", połowa parsująca staje się testowalna na zapisanych fixture'ach — czego dziś w ogóle nie da się zrobić. Punkt (c) (zapis raw HTML) i ten seam wzmacniają się nawzajem: raw HTML zapisany w bazie **jest** naturalnym źródłem fixture'ów.
- **Normalizacja dat w Go zamiast w JS**. Dziś data jest składana do ISO stringiem w kodzie JS (`m[3] + '-' + m[2] + '-' + m[1]`), a `optDate` tylko parsuje. Przy per-portal ekstraktorach lepiej wyciągać surowy tekst i robić całą normalizację w Go — jeden parser na format portalu, w pełni testowalny.
- **`optText` do ponownego użycia bez zmian** — jest portal-agnostyczne.
- **`WithTx` z sqlc już istnieje, ale jest nieużywane** — dostępne, gdyby zapis raw HTML + keywords wymagał atomowości.
- **Dodanie kolumny `source` otwiera filtr źródła w UI** (`?source=nofluffjobs` lub linki w `categorynav.gohtml`) — poza zakresem, ale naturalne następstwo.
- **Naprawa dokumentacji przy okazji**: `tech-stack.md` podaje nieistniejący target `make sqlc-generate` (jest `make sqlc`) i twierdzi, że "no migration framework needed", co nie uwzględnia mechanizmu `deploy.sh`.

---

## Ocena wpływu

### Zmiany główne

- `internal/category/category.go` — przebudowa lub zastąpienie modelem ładowanym z bazy; dodanie dyskryminatora portalu.
- `internal/justjoin/fetcher.go` — dispatch strategii dla odkrywania linków; nowe stałe/konfiguracja `frequency`; przewleczenie identyfikatora portalu przez `FetchedOffers`.
- `internal/justjoin/transformer.go` — wyjęcie ekstrakcji justjoin.it do osobnej strategii; dispatch per portal; nowe pauzy; przechwycenie raw HTML; wypełnienie `Source`/`RawHtml`/`Keywords`.
- **Nowe pliki**: `internal/justjoin/justjoin_scraper.go`, `nofluffjobs_scraper.go`, `bulldogjob_scraper.go`, `solidjobs_scraper.go` (snake_case, po jednym na portal, zgodnie z idiomem repozytorium).
- `internal/database/schema.sql` — tabela źródeł + kolumny `source`/`raw_html`/`keywords` w `possibilities`.
- `internal/database/queries/possibilities.sql` — rozszerzenie `CreatePossibility` o nowe kolumny (**bez ruszania `ON CONFLICT (url)` i `ExistingPossibilityUrls`**); nowy plik zapytań dla źródeł.
- `internal/database/generated/*` — regeneracja przez `make sqlc`.
- `scripts/deploy.sh` — nowy idempotentny blok DDL dla istniejącej bazy produkcyjnej.
- `cmd/crawler/main.go` — ładowanie źródeł z bazy, budowa mapy `portal → strategia`.

### Zmiany powiązane

- `internal/handlers/helper.go`, `stats_handler.go`, `scraped_handler.go`; `internal/templates/parse.go` — dostosowanie do źródeł ładowanych z bazy zamiast `category.All`.
- `internal/justjoin/persist.go` — rewizja timeoutu 3 s pod zapis raw HTML.
- `internal/config/loader.go` + `.env.default` + `docker-compose.yaml` — jeśli throttling ma być konfigurowalny.
- `internal/templates/scraped/list.gohtml`, `partials/categorynav.gohtml` — opcjonalna kolumna/filtr źródła (**raw HTML nie może trafić do listy**).
- `.maister/docs/project/tech-stack.md` — poprawienie nieaktualnych zapisów.

### Aktualizacje testów

- Nowe testy tabelaryczne dla parserów per portal (możliwe dopiero po wprowadzeniu seamu parsowania; wymaga wprowadzenia konwencji fixture'ów HTML, której dziś nie ma).
- **Rekomendowany test integracyjny `CreatePossibility` na prawdziwym Postgresie** weryfikujący, że `ON CONFLICT (url)` nadal deduplikuje po dodaniu kolumn — wzorzec self-skip jak w `redis_session_repository_test.go`.
- Rozszerzenie `TestUnseen` o scenariusz wieloportalowy (te same tytuły, różne URL-e — muszą być traktowane jako różne oferty, zgodnie z wyłączeniem deduplikacji semantycznej z zakresu).
- Test parsowania dat per format portalu (po przeniesieniu normalizacji do Go).

### Poziom ryzyka: Wysoki

Składowe: (1) zmiana schematu produkcyjnej bazy bez frameworka migracji i bez ścieżki rollbacku; (2) niepotwierdzone w repozytorium założenie o schemacie `wok`; (3) zero testów chroniących mechanizm deduplikacji objęty zakazem regresji; (4) trzy nieznane portale o nieznanym DOM-ie, modelu paginacji i postawie anty-botowej; (5) nowy throttling czyniący pełny przebieg wielokrotnie dłuższym niż interwał między przebiegami. Ryzyko *typów Go* jest niskie (kompilator złapie braki), ryzyko *danych i operacyjne* — wysokie.

---

## Rekomendacje

Zadanie to **rozbudowa istniejącego kodu** (istnieje działająca implementacja dla jednego portalu), więc rekomendacje dotyczą strategii wdrożenia, kompatybilności wstecznej i testów.

### Przed implementacją — decyzje wymagające potwierdzenia użytkownika

1. **URL-e portali** — wymóg jawnie postawiony w zadaniu; trzy nowe portale, ich listingi per kategoria, model paginacji.
2. **Kształt źródła w bazie**: pełna tabela `sources` (id, name, base_url, aktywność) z FK z `possibilities` i URL-ami listingów per kategoria — vs. denormalizowana kolumna `source TEXT`. Punkt (a) sugeruje tabelę; `models.md` ją popiera, `minimal-implementation.md` każe nie przesadzać. **Decyzja użytkownika.**
3. **Definicja `keywords`**: scrapowane z portalu / wyprowadzone z `tech` + `description` / generowane? Typ: `TEXT[]` czy `TEXT` rozdzielany przecinkami (jak istniejące `tech`, którego `StatsPossibilityByCategory` używa przez `string_to_array`)?
4. **Polityka `raw_html`**: przechowywać w nieskończoność czy z retencją? Kompresować? Osobna tabela (żeby nie puchła gorąca ścieżka `ListPossibilitiesByCategory`)? Rekomendacja: **osobna tabela `possibility_raw_html` z FK 1:1** — utrzymuje `possibilities` szczupłe dla listingu i statystyk oraz pozwala niezależnie zarządzać retencją.
5. **Budżet przepustowości**: przy 2-3 min/oferta pełny przebieg po 4 portalach × 5 kategoriach przekroczy interwał 4-5 h. Trzeba ustalić limit ofert na przebieg lub równoległość per portal.

### Strategia implementacji

- **Kolejność**: (d) → (a) → (c) → (b). Punkt (d) jest samodzielny i natychmiast obniża ryzyko banów przy dalszych eksperymentach. (a) i (c) to zmiany schematu — zrobić je razem, w jednym bloku DDL, przed dotknięciem strategii. (b) na końcu, portal po portalu, z justjoin.it jako pierwszym refaktorem "bez zmiany zachowania".
- **Punkt (b) — kształt abstrakcji.** Powielić idiom `IpRepository`: mały interfejs zadeklarowany **w pakiecie `crawler`** (konsumencie), o metodach będących dokładnie tym, co `fetcher.go`/`transformer.go` już wywołują — mniej więcej `FetchOfferURLs(ctx, source) ([]string, error)` i `ScrapeOffer(ctx, url) (params, error)`. Jeden plik snake_case na portal, `New<Portal>Scraper(...) *<Portal>Scraper`. Ponieważ to relacja 1:wiele (nie zamiana 2-stronna), literalna `map[string]offerScraper` zbudowana raz w `main()` jest minimalnym rozszerzeniem idiomu. **Nie budować** frameworka ekstraktorów, pipeline'ów pluginowych, map selektorów sterowanych konfiguracją ani auto-rejestracji — `minimal-implementation.md` tego zabrania, a przy 4 portalach nie ma to uzasadnienia.
- **Uzasadnienie abstrakcji względem standardów**: `minimal-implementation.md` zakazuje abstrakcji "unless there's an immediate need". Przy czterech konkretnych portalach potrzeba jest **natychmiastowa, nie hipotetyczna** — abstrakcja jest uzasadniona. Ograniczenie dotyczy jej *rozmiaru*, nie jej istnienia.
- **Wprowadzić seam pobranie/parsowanie**: strategia powinna oddzielić "przywieź raw HTML z chromedp" od "wyciągnij pola z HTML-a". To jednocześnie realizuje punkt (c), czyni parsowanie testowalnym i pozwala re-parsować zapisany HTML bez ponownego crawlowania.
- **Przenieść całą normalizację dat do Go** — po jednym parserze na format portalu, zamiast składania ISO stringów w kodzie JS.
- **Nie kopiować anty-wzorców**: `strings.Contains(body, "Remote")`, lokalizacja jako "linia po tytule w innerText", monolityczny `chromedp.Run`, ciche `""` przy zepsutym selektorze. Nowe ekstraktory powinny sięgać po rzeczywiste węzły DOM i **logować**, gdy oczekiwane pole nie zostało znalezione (rozróżnienie "selektor padł" vs "pola nie ma").

### Kompatybilność wsteczna

- **Deduplikacja po URL musi zostać nietknięta** (jawny zakaz w zakresie). Konkretnie: **nie zmieniać** klucza `UNIQUE (url)` ani celu `ON CONFLICT (url)`, i **nie ruszać** `ExistingPossibilityUrls`. Nowe portale mają globalnie różne URL-e absolutne, więc gołe `url` nadal deduplikuje poprawnie; kolumnę `source` dodać **obok**, nie do klucza. Gdyby kiedykolwiek pojawił się pomysł `UNIQUE(source_id, url)` — oba miejsca musiałyby zmienić się jednocześnie, inaczej deduplikacja cicho pęknie.
- **Istniejące wiersze**: nowe kolumny muszą być nullable albo mieć `DEFAULT` — istniejące oferty justjoin.it nie mają `source`, `raw_html` ani `keywords`. Rozważyć backfill `source = 'justjoin'` osobnym, jednorazowym poleceniem (`migrations.md`: "Separate Schema and Data").
- **DDL musi być schema-agnostyczny**: używać `current_schema()` (jak istniejący guard w `deploy.sh`), nigdy `public`. Rekomendacja: **dodać do `deploy.sh` asercję sprawdzającą, że `current_schema()` to `wok`, przed zastosowaniem DDL** — dziś to niezweryfikowane założenie oparte na ręcznej konfiguracji VPS-a.
- **Zaktualizować oba miejsca**: `schema.sql` (świeże instalacje + wejście dla sqlc) **oraz** blok heredoc w `scripts/deploy.sh` (istniejąca baza). Pominięcie któregokolwiek daje rozjazd między środowiskami.
- **Zgłosić użytkownikowi lukę wobec standardów**: mechanizm `deploy.sh` narusza `migrations.md` w czterech punktach. Podążać za istniejącą konwencją (spójność), ale jawnie zaproponować wprowadzenie prawdziwego mechanizmu migracji jako osobne zadanie.

### Strategia testowa

- Przed refaktorem (b) dopisać **test charakteryzujący** dla obecnego zachowania justjoin.it, żeby refaktor był weryfikowalnie bezpieczny.
- **Test integracyjny na prawdziwym Postgresie** dla `CreatePossibility` — potwierdzający, że upsert po `url` nadal działa po dodaniu kolumn. Wzorzec self-skip z `redis_session_repository_test.go` (`t.Skipf` gdy baza nieosiągalna) pasuje idealnie.
- Testy tabelaryczne per portal dla warstwy parsowania, na fixture'ach HTML — **wprowadzić konwencję fixture'ów dopiero wtedy, gdy refaktor faktycznie utworzy ten seam** (zgodnie z `minimal-implementation.md`).
- Trzymać się idiomu repo: stdlib `testing`, tabele + `t.Run`, ręczne stuby, bez frameworków mockujących.

---

## Następne kroki

1. **Przedstawić użytkownikowi 5 decyzji do potwierdzenia** z sekcji "Przed implementacją" — w szczególności URL-e portali (jawny wymóg zadania), kształt źródła w bazie, definicję `keywords` i budżet przepustowości przy nowym throttlingu.
2. **Zgłosić dwa krytyczne ustalenia infrastrukturalne**: brak frameworka migracji (luka wobec `standards/backend/migrations.md`) oraz nieobecność schematu `wok` w kontroli wersji.
3. **Uruchomić gap-analyzer**, żeby zestawić stan obecny z docelowym i zamienić powyższe w listę konkretnych luk do domknięcia w specyfikacji.
