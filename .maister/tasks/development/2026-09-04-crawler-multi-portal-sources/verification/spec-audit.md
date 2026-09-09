# Audyt specyfikacji: crawler wieloportalowy

**Zadanie:** `2026-09-04-crawler-multi-portal-sources`
**Data audytu:** 2026-09-04
**Audytowany dokument:** `implementation/spec.md` (949 wierszy — spec deklaruje 947)
**Metoda:** niezależna weryfikacja każdego cytatu `plik:linia` w rzeczywistym kodzie; `go build ./...`, `go vet ./...`, `go test ./...` uruchomione na stanie wyjściowym (wszystkie zielone, `77878d2`).

---

## Werdykt

**`pass-with-concerns`**

| Waga | Liczba |
|---|---|
| Krytyczne | 0 |
| Wysokie | 5 |
| Średnie | 7 |
| Niskie | 7 |
| **Razem** | **19** |

Specyfikacja jest nietypowo dobra na tle tego, co zwykle trafia do audytu: **21 z 24 zweryfikowanych cytatów `plik:linia` zgadza się co do znaku**, decyzje architektoniczne są uzasadnione i skonfrontowane ze standardami, a świadomie przyjęte długi są odnotowane rzetelnie (włącznie z tymi, które są dla autora niewygodne). Spec **poprawił** dwa błędne cytaty odziedziczone z wcześniejszych artefaktów (`persist.go:34` → `:32`, `transformer.go:166-225` → `:153-247`).

Nie jest to jednak dokument gotowy do przekazania planiście bez poprawek. Znaleziono pięć ustaleń wysokiej wagi, z czego dwa to **mechaniczne wady kryteriów akceptacji** (kryteria 14 i 24 nie testują tego, co deklarują), jedno to **fałszywa przesłanka techniczna** przeniesiona z Fazy 2, jedno to **niekompletna analiza ripple'u**, a jedno to **funkcjonalna konsekwencja F5, której żaden artefakt nie dostrzegł**.

Rekomendacja: naprawić ustalenia H1-H5 i M1-M3 przed fazą planowania. Reszta może zostać rozstrzygnięta przy implementacji.

---

## 1. Weryfikacja punkt po punkcie — sekcja „NIE UFAJ SPECYFIKACJI"

| # | Twierdzenie specyfikacji | Werdykt | Faktyczny stan |
|---|---|---|---|
| 1a | crawler zapisuje `cat.Name` (`fetcher.go:73`) | **POTWIERDZONE** | `fetcher.go:73`: `offers := FetchedOffers{Category: cat.Name, Urls: of.getUrls(...)}` |
| 1b | `scraped_handler.go:55` odpytuje po `Name` | **POTWIERDZONE** | `scraped_handler.go:55`: `h.store.ListPossibilitiesByCategory(ctx, cat.Name)` |
| 1c | `stats_handler.go:53` odpytuje po `Name` | **POTWIERDZONE** | `stats_handler.go:53`: `h.store.StatsPossibilityByCategory(ctx, cat.Name)` |
| 2 | `cdp.Node.Attribute("href")` w `fetcher.go:208`, wartość bez zmian do `possibilities.url` | **POTWIERDZONE** | `fetcher.go:208` → `links.add(href)` (`:209`) → `links.urls` (`:159`) → `getUrls` (`:114`) → `batch.Urls` → `unseen` (`transformer.go:120`) → `scrapeOffer(browserCtx, url, ...)` → `Url: url` (`transformer.go:233`). **Zero normalizacji na całej ścieżce.** Decyzja `url-canonicalization` ma solidną podstawę |
| 3 | `colspan="4"` w `scraped/list.gohtml` | **POTWIERDZONE** | `scraped/list.gohtml:29` — dokładnie ta linia, którą spec podaje |
| 4a | `possibilities.url TEXT NOT NULL UNIQUE` | **POTWIERDZONE** | `schema.sql:24` |
| 4b | `ON CONFLICT (url) DO UPDATE SET posted_at = COALESCE(...)` | **POTWIERDZONE** | `queries/possibilities.sql:17-18` — spec przepisuje klauzulę znak w znak |
| 4c | `possibilities_category_created_at_idx (category, created_at DESC, id DESC)` | **POTWIERDZONE** | `schema.sql:36-37` |
| 4d | `ListPossibilitiesByCategory` to dziś `SELECT *` | **POTWIERDZONE** | `queries/possibilities.sql:4-7` |
| 4e | DDL specyfikacji spójne z resztą schematu (`BIGSERIAL`, `TIMESTAMPTZ`) | **POTWIERDZONE** | `schema.sql:3-34` — `BIGSERIAL PRIMARY KEY`, `TIMESTAMPTZ NOT NULL DEFAULT now()` to istniejąca konwencja |
| 5a | mechanizm heredoc w `deploy.sh` | **POTWIERDZONE** | `deploy.sh:55` — `$DOCKER compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 ...' <<'SQL'` |
| 5b | `current_schema()` w `deploy.sh:62` | **POTWIERDZONE** | `deploy.sh:62`, w guardzie `DO $$` — dokładnie wzorzec, który spec chce powielić |
| 5c | proponowany runbook da się nałożyć na `deploy.sh` | **NIE POTWIERDZONE** | patrz **H1** i **H2** — mechanizm dostarczenia pliku migracji do kontenera nie istnieje i nie jest opisany |
| 6a | `scrapeOffer` w `transformer.go:153-247` | **POTWIERDZONE** | dokładnie `:153-247` |
| 6b | `pause()` w `transformer.go:135-145` | **POTWIERDZONE** | dokładnie `:135-145` |
| 6c | `optDate`/`optText` w `transformer.go:251-266` | **POTWIERDZONE** | dokładnie `:251` i `:260-266` |
| 6d | stałe throttlingu w `fetcher.go:16-17` i `transformer.go:20-21` | **POTWIERDZONE** | `frequency`/`maxFrequencyJitter` na `:16-17`, `minOfferPause`/`maxOfferPause` na `:20-21` |
| 6e | anty-wzorce: `strings.Contains(body,"Remote")` `:242`, lokalizacja `:176-186`, data w JS `:218-224`, ciche `""` `:150-152` | **POTWIERDZONE** | wszystkie cztery cytaty trafiają |
| 7a | `Querier` + `var _ Querier = (*Queries)(nil)` | **POTWIERDZONE** | `generated/querier.go:11-22`, asercja `:24`; `emit_interface: true` w `sqlc.yaml` |
| 7b | `DBTX` bez `Begin` | **POTWIERDZONE** | `generated/db.go:14-18` — tylko `Exec`/`Query`/`QueryRow`. `WithTx(tx pgx.Tx)` istnieje (`:28`), ale nie ma czym otworzyć transakcji przez `Queries`. Uzasadnienie „`WithTx` zostaje nieużywane" jest poprawne |
| 7c | typy `pgtype.*` w modelach | **POTWIERDZONE** | `generated/models.go:77-89`; `Possibility` dostanie `SourceID pgtype.Int8` zgodnie z konwencją `pgtype.Int4` na `:63-65` |
| 7d | „łamią się dokładnie dwa miejsca" | **OBALONE** | patrz **M1** — trzecie miejsce to `internal/router/routing_test.go:118` |
| 8a | `sync.WaitGroup.Go` w `cmd/crawler/main.go` | **POTWIERDZONE** | `main.go:40` (`wg := sync.WaitGroup{}`), `:43`, `:48`, `:53`. `go.mod:3` → `go 1.26`, więc `WaitGroup.Go` (Go 1.25+) jest dostępne. Pętla `for ... range` po literale slice'a jest bezpieczna przy semantyce zmiennych pętli Go 1.22+ |
| 8b | proponowane wiązanie 4 pipeline'ów da się nałożyć | **CZĘŚCIOWO** | struktura goroutine'ów tak; brakuje właściciela mapowania `[]database.ListEnabledScrapeTargetsRow` → `map[string][]crawler.ScrapeTarget` (**M5**) |
| 9 | badge `text-bg-light border` w `offers/list.gohtml:40,44` | **POTWIERDZONE** | dokładnie `:40` i `:44`; idiom `{{ if eq }}` w `<td>` na `:36-45`. Decyzja „zero nowego CSS" ma pokrycie |

**Podsumowanie weryfikacji cytatów:** 21 potwierdzonych dokładnie, 1 obalony (7d), 2 częściowe (5c, 8b). Dodatkowo zweryfikowano pozytywnie 12 cytatów z sekcji 11.1 (`transformer.go:28`, `:100-130`, `persist.go:11`, `router/routing.go:50-58`, `browser.go:20,47`, `fetcher.go:50,78-82,110,144-160`, `helper.go:36,64`, `scraped/list.gohtml:8-10,13,22,23,29`, `scraped_handler.go:14-16,28-33`, `templates/parse.go:13`) — wszystkie trafiają. Trzy cytaty są błędne — patrz **L1-L2**.

---

## 2. Ustalenia wysokiej wagi

### H1. Brak mechanizmu dostarczenia `001_multi_portal_sources.sql` do bazy

**Odniesienie:** spec §7.1 („Uruchamiany przez `scripts/deploy.sh`"), §7.3 krok 5, §11.2.

**Dowód:** `deploy.sh:55` wykonuje SQL **wyłącznie** przez heredoc z hosta:
```
$DOCKER compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" "$POSTGRES_DB"' <<'SQL'
```
Kontener `postgres` ma dziś jeden bind mount z repozytorium (`docker-compose.yaml:15` → `schema.sql`). Katalog `internal/database/migrations/` **nie jest nigdzie montowany**, więc `psql -f internal/database/migrations/001_...sql` wykonane wewnątrz kontenera nie znajdzie pliku.

**Kategoria:** Missing (luka w projekcie dostarczenia)

**Skutek:** implementujący ma trzy niezgodne opcje i spec nie wskazuje żadnej:
1. `... psql ...' < internal/database/migrations/001_multi_portal_sources.sql` (przekierowanie z hosta) — działa, ale traci `\d` i wymaga zmiany kształtu wywołania,
2. nowy bind mount w `docker-compose.yaml`,
3. wklejenie treści migracji do heredoca — **łamie deklarowaną w §7.1 zasadę append-only i DRY**, bo plik istnieje wtedy w dwóch miejscach.

Dodatkowo spec nie mówi, co dzieje się z **istniejącym** blokiem heredoc (`deploy.sh:56-78`: cast `posted_at`, `DROP INDEX`, `CREATE INDEX`) — zostaje obok, zostaje wchłonięty do `001`, czy znika.

**Rekomendacja:** dopisać do §7.1 jawny kształt wywołania w `deploy.sh` i rozstrzygnąć los dotychczasowego bloku. To jedna linia decyzji, ale bez niej dwóch implementujących odda dwie różne architektury deployu.

---

### H2. Kroki danych 3 i 6 runbooka nie mają przypisanego miejsca wykonania

**Odniesienie:** spec §7.3 kroki 3 i 6, §12 („Separate Schema and Data — DDL osobno od `seed.sql` i od obu kroków `UPDATE`").

**Dowód:** `deploy.sh:13` ma `set -euo pipefail` i jest w całości nieinteraktywny; uruchamiany na każdym wdrożeniu (`Makefile:32-33`). Krok 3 (normalizacja URL-i) jest z definicji **warunkowy** i zależy od ręcznego wyniku zapytania z §4. Krok 6 (backfill `source_id`) jest idempotentny, ale też nie ma pliku.

Tabela plików w §7.1 wymienia **trzy** pliki: `schema.sql`, `001_multi_portal_sources.sql`, `seed.sql`. Żaden z nich nie może zawierać kroków `UPDATE` bez złamania reguły, którą §12 sam sobie stawia.

**Kategoria:** Missing

**Skutek:** kroki 3 i 6 są opisane jako część runbooka, ale nie są ani plikiem w repozytorium, ani blokiem w `deploy.sh`, ani jawnie oznaczone jako operacja ręczna przez `psql`. Krok 6 jest przy tym warunkiem sensowności całej kolumny `Source` od pierwszego dnia (rozstrzygnięcie kwestii otwartej 8).

**Dodatkowo — kolejność:** krok 1 (zapytania weryfikacyjne na produkcji) musi się wykonać **przed** `deploy.sh:32` (`compose down`), bo między `:32` a `:41` baza nie działa. Spec numeruje krok 1 przed krokiem 2 („`deploy.sh` → `make db-backup`"), co sugeruje, że oba są w tym samym przebiegu skryptu — a nie są.

**Rekomendacja:** jawnie oznaczyć kroki 1, 3 i 6 jako operacje ręczne wykonywane przez `make db-restore`-podobne wywołanie `psql`, albo dodać czwarty plik (`internal/database/migrations/001_..._data.sql`) i wskazać moment jego uruchomienia.

---

### H3. Kryterium akceptacji 14 zwróci fałszywy alarm natychmiast po wdrożeniu

**Odniesienie:** spec §14, kryterium 14.

**Dowód:** kryterium to
```sql
SELECT count(*) FROM possibilities p
LEFT JOIN possibility_html h ON h.possibility_id = p.id
WHERE p.source_id IS NOT NULL AND h.possibility_id IS NULL;
```
z oczekiwanym wynikiem `0`. Ale krok 6 runbooka (§7.3) ustawia `source_id` **wszystkim** historycznym wierszom justjoin.it:
```sql
UPDATE possibilities SET source_id = (SELECT id FROM sources WHERE slug = 'justjoin')
WHERE source_id IS NULL AND url LIKE 'https://justjoin.it/%';
```
Te wiersze nie mają i nigdy nie będą miały HTML-a (tabela `possibility_html` powstaje dopiero w tym wdrożeniu, a `unseen` nie pozwoli ich ponownie zescrapować — patrz **H4**). Zaraz po deployu kryterium 14 zwróci **całą historyczną liczbę ofert**, nie `0`.

**Kategoria:** Incorrect

**Skutek:** jedyny mechanizm wykrywania sierot po odrzuconej transakcji (§15.2, dług nr 1) jest bezużyteczny w postaci, w jakiej został zapisany. Spec sam mówi: „Wartość `> 0` oznacza sieroty po nieudanym drugim `INSERT`-cie" — co po backfillu jest po prostu nieprawdą.

**Rekomendacja:** dodać guard, np. `AND p.created_at > :deploy_timestamp`, albo `AND p.id > (SELECT max(id) FROM possibilities WHERE created_at < ...)`. Bez tego kryterium jest niewykonalne.

---

### H4. Wiersze z placeholderem `(unparsed)` są nieodwracalne — spec tego nie odnotowuje

**Odniesienie:** spec §9.3 (polityka F5), §5.3 (zakaz regresji), §14 kryterium 15.

**Dowód:** trzy mechanizmy razem tworzą pułapkę, której żaden artefakt nie opisuje:
1. `unseen` (`transformer.go:100-130`) odsiewa każdy URL obecny w `possibilities` — również ten z `job_title = '(unparsed)'`. Oferta nigdy nie zostanie ponownie zescrapowana.
2. Nawet gdyby została, `ON CONFLICT (url) DO UPDATE SET posted_at = COALESCE(...)` (`queries/possibilities.sql:17-18`) **nie dotyka** `job_title` ani `company_name` — i spec §5.3 jawnie zakazuje zmiany tej klauzuli.
3. Zapisany HTML jest, ale nic w aplikacji go nie czyta (F4: tabela write-only).

**Kategoria:** Ambiguous / brakująca konsekwencja

**Skutek:** to nie jest przypadek brzegowy — to **spodziewany scenariusz pierwszych dni**. Trzy nowe portale dostają parsery pisane na kilku fixture'ach; każda oferta, na której parser się potknie, zostaje w bazie na stałe jako `(unparsed)`, a jedyna droga naprawy to ręczny `DELETE FROM possibilities WHERE job_title = '(unparsed)'` przed kolejnym przebiegiem. Spec nie mówi o tym słowa, mimo że §15 skrupulatnie kataloguje mniej prawdopodobne ryzyka.

Zauważalna jest przy tym asymetria: spec obszernie uzasadnia, dlaczego zapisany HTML jest „jedyną drogą do późniejszego odzyskania takiej oferty" (§9.3), ale odzyskanie to jest możliwe wyłącznie poza aplikacją, ręcznie, bez żadnego wsparcia — a UI oznacza taki wiersz kursywą i na tym koniec.

**Rekomendacja:** dopisać do §15.2 jako świadomy dług wraz z procedurą naprawczą (`DELETE` po URL-ach z `(unparsed)`), albo — tańszy wariant — dodać do §9.4 uwagę, że `unseen` mógłby pomijać wiersze z placeholderem. Drugi wariant to jedna klauzula `AND job_title <> '(unparsed)'` w `ExistingPossibilityUrls`, ale ta metoda jest objęta zakazem regresji, więc decyzja należy do użytkownika.

---

### H5. `knownOffersTimeout = 5 s` zostaje bez zmian mimo tej samej przesłanki, która podniosła timeout persist 20×

**Odniesienie:** spec §9.6, tabela „Pozostałe timeouty" — wiersz `knownOffersTimeout`, **kolumna „Powód" jest pusta**.

**Dowód:** spec podnosi timeout zapisu z 3 s na 60 s (`persist.go:32`) i uzasadnia to wprost: „cztery pipeline'y konkurujące o pulę `pgxpool` bez jawnej konfiguracji rozmiaru". Ta sama przesłanka dotyczy `knownOffersTimeout` (`transformer.go:24`), który zostaje na 5 s.

Ścieżka awarii jest krótka i w całości udokumentowana w kodzie:
- `internal/config/loader.go:55-60` buduje DSN z jedynym parametrem `sslmode=disable` — **`pool_max_conns` nie da się dziś ustawić bez zmiany kodu**; domyślnie `max(4, NumCPU)`.
- przy wysyceniu puli `ExistingPossibilityUrls` przekracza 5 s,
- `transformer.go:108-112`: przy błędzie lookup zwraca **cały batch** („scraping all of them"),
- transformer scrapuje do 100 znanych ofert × 2-3 min = **~4 h bezcelowego ruchu na portalu**.

To jest dokładnie ten scenariusz, przed którym §15.1 ostrzega w wierszu „Zapis `location.href` po przekierowaniu" („każdy przebieg scrapuje wszystko od nowa → prosta droga do bana") — tyle że wyzwalany przez timeout bazy, nie przez URL.

**Kategoria:** Incomplete

**Skutek:** ryzyko bana z powodu chwilowej niedostępności puli połączeń, przy limicie 100 ofert kosztujące pół doby ruchu na czterech portalach.

**Rekomendacja:** albo podnieść `knownOffersTimeout` razem z timeoutem persist (koszt: jedna stała), albo wypełnić pustą komórkę „Powód" świadomym uzasadnieniem. Powiązać z kwestią otwartą 6 — patrz sekcja 6.

---

## 3. Ustalenia średniej wagi

### M1. Analiza ripple'u jest niekompletna — trzecie miejsce łamania

**Odniesienie:** spec §8.3: „konsumenci deklarują wąskie interfejsy, więc **łamią się dokładnie dwa miejsca** i kompilator wskaże oba".

**Dowód:** `internal/router/routing_test.go:118`
```go
func (s *fakePossibilityStore) ListPossibilitiesByCategory(_ context.Context, _ string) ([]database.Possibility, error) {
```
Ten stub implementuje `handlers.PossibilityStore` i jest przekazywany do `handlers.NewScrapedHandler` w `routing_test.go:130`. Po zmianie typu zwracanego na `[]database.ListPossibilitiesByCategoryRow` przestanie spełniać interfejs.

**Kategoria:** Incomplete

**Skutek:** ograniczony — `go test ./...` (kryterium 21) to wychwyci. Ale twierdzenie „dokładnie dwa" jest fałszywe, a `go build ./...` **nie** wykryje tego miejsca (plik `_test.go`), więc implementujący opierający się na „kompilator wskaże oba" zobaczy zieloną kompilację i czerwone testy dopiero na końcu grupy zadań.

**Rekomendacja:** poprawić §8.3 na trzy miejsca i wymienić `routing_test.go:118`.

---

### M2. Fałszywa przesłanka: `disable-dev-shm-usage` jest już włączone

**Odniesienie:** spec §7.2 („`shm_size` **nie jest dodawane** — zastępuje je `chromedp.Flag("disable-dev-shm-usage", true)` w `newBrowser`"), §9.5 („domyślne `/dev/shm` = 64 MB przy ośmiu instancjach niemal gwarantuje awarie kart"), §11.1. Przesłanka odziedziczona z `scope-clarifications.md` („omija domyślne 64 MB `/dev/shm`") i `requirements.md` F7.

**Dowód:** `internal/justjoin/browser.go:21-24` buduje allokator przez `append(chromedp.DefaultExecAllocatorOptions[:], chromedp.NoSandbox)`. W `chromedp@v0.16.0`, `allocate.go:69`, `DefaultExecAllocatorOptions` zawiera już:
```go
Flag("disable-dev-shm-usage", true),
```
**Flaga jest aktywna od pierwszego dnia istnienia crawlera.**

**Kategoria:** Incorrect

**Skutek:** dwojaki.
1. Proponowana zmiana w `newBrowser` jest **martwym kodem** — duplikuje flagę już ustawioną. Łamie `minimal-implementation.md` („Unused Code Is Debt") i `coding-style.md` (DRY), które spec deklaruje jako spełnione w §12.
2. Poważniejsze: ryzyko opisane w §9.5 („niemal gwarantuje awarie kart") **nie istnieje w podanej postaci**, bo jest już zmitygowane. Jeśli przy ośmiu instancjach Chromium wystąpią awarie „Target closed", diagnoza pójdzie w złą stronę, bo spec wskazuje jako przyczynę coś, co jest wykluczone. Prawdziwym wąskim gardłem jest wtedy RAM i `/tmp`, nie `/dev/shm`.

**Rekomendacja:** usunąć zmianę w `newBrowser` z zakresu, przeformułować uzasadnienie `mem_limit: 4g` (samo w sobie zasadne) i odnotować, że flaga pochodzi z `DefaultExecAllocatorOptions`.

---

### M3. Kryterium akceptacji 24 nie testuje tego, co deklaruje

**Odniesienie:** spec §14, kryterium 24: „Grep po `chromedp` w `internal/handlers` i `cmd/web` nie daje trafień (stała placeholdera nie wciągnęła zależności przeglądarkowych do binarki web)".

**Dowód:** grep po tekście źródłowym nie wykrywa zależności **tranzytywnej**. Gdyby `internal/handlers` zaimportowało `internal/justjoin`, w plikach `internal/handlers` nadal nie padłoby słowo `chromedp` — a `chromedp` i tak trafiłby do binarki `cmd/web`. Kryterium przechodzi dziś i przeszłoby również w scenariuszu, przed którym ma chronić.

**Kategoria:** Incorrect

**Rekomendacja:** zamienić na sprawdzenie faktycznego grafu zależności:
```
go list -deps ./cmd/web | grep -q chromedp && echo FAIL || echo OK
```
To jedyne kryterium uzasadniające istnienie osobnego pakietu `internal/offer` (§9.3), więc powinno działać.

---

### M4. Logika retry z fetchera nie ma przypisanego miejsca w nowej architekturze

**Odniesienie:** spec §9.2 („Wspólne helpery pakietu"), §9.4 (przepływ).

**Dowód:** `fetcher.go:86-115` (`getUrls`) zawiera pętlę retry: `maxFetchAttempts = 3` (`:20`), `fetchRetryBaseDelay = 2 * time.Second` (`:21`), wykładniczy backoff `time.Sleep(backoff)` (`:103`). Wraz z `scrapeOnce` (`:117-138`) tworzy warstwę między `fetchAll` a zbieraniem linków.

**Grep po całej specyfikacji i po `gap-analysis.md`: `maxFetchAttempts`, `fetchRetryBaseDelay`, `scrapeOnce`, `getUrls`, `scrollStepDelay` — zero trafień.** §9.2 wymienia, co dzieje się ze `scrollAndCollectLinks`, `collectOfferLinks` i `orderedLinks`, ale pomija całą warstwę retry.

**Kategoria:** Missing

**Skutek:** dwie niejednoznaczności naraz:
- czy retry zostaje w `OfferFetcher` (wokół `collector.CollectOfferURLs`), czy każdy kolektor retry'uje u siebie? Pierwsze jest DRY, drugie daje portalowi kontrolę nad tym, co jest błędem przejściowym.
- `time.Sleep(backoff)` na `fetcher.go:103` jest **nieanulowalne**. Dziś to maksymalnie 6 s opóźnienia shutdownu; przy czterech fetcherach to do 4 × 6 s. Spec bardzo starannie opisuje graceful shutdown (§9.5) i kryterium 10 wymaga „bez zawieszenia" — a jedyne nieanulowalne czekanie w pakiecie nie jest wymienione. W repozytorium istnieje gotowy `pause(ctx, d)` (`transformer.go:135`), którego spec sam wskazuje jako element do ponownego użycia.

**Rekomendacja:** dopisać do §9.2 właściciela retry i zamienić `time.Sleep` na `pause(ctx, backoff)`.

---

### M5. Kontrakt `LinkCollector` nie mówi, kto jest właścicielem karty przeglądarki

**Odniesienie:** spec §9.1 (`CollectOfferURLs(ctx context.Context, listingURL string)`), §9.4 (przepływ: `collector.CollectOfferURLs(tabCtx, target.URL)`), §9.2 (Bulldogjob: paginacja 3 stron).

**Dowód:** sygnatura interfejsu przyjmuje gołe `ctx context.Context` bez komentarza o tym, czym ten kontekst jest. Diagram przepływu nazywa argument `tabCtx`, co sugeruje, że fetcher otwiera kartę przez `newTab(browserCtx, listingTimeout)` (`browser.go:47`) i wręcza ją kolektorowi. Komentarz `newTab` (`browser.go:42-46`) ostrzega przy tym wprost: „Sharing one tab across pages means the first timeout kills every page after it".

**Kategoria:** Ambiguous

**Skutek:** dwóch implementujących rozjedzie się na starcie:
- wariant A (fetcher otwiera kartę): kolektor Bulldogjoba nawiguje trzy strony **w jednej karcie z jednym budżetem 3 minut**,
- wariant B (kolektor dostaje `browserCtx` i otwiera kartę na stronę): każda strona ma własny `listingTimeout`.

To nie jest kosmetyka — od tego zależy, czy **kwestia otwarta 4** („czy podnieść `listingTimeout`") w ogóle ma sens w postaci, w jakiej została zadana. Patrz sekcja 6.

**Rekomendacja:** dopisać jedno zdanie do komentarza interfejsu w §9.1: czy `ctx` to kontekst przeglądarki, czy gotowa karta.

---

### M6. Lista targetów czytana raz przy starcie — kolizja z historyjką użytkownika 4

**Odniesienie:** spec §2 historyjka 4 („chcę włączać i wyłączać pojedyncze pary portal × kategoria przez `psql`, **bez rekompilacji i redeploya binarki**"), §8.1 („Jedno zapytanie **przy starcie crawlera**"), §10.4.

**Dowód:** `ListEnabledScrapeTargets` wywoływane jest raz, w `main()`, przed pętlą wiązania (§9.5). Zmiana `enabled` przez `psql` nie ma żadnego wpływu na działający proces.

Warto zauważyć wewnętrzną niespójność: §10.4 odrzuca mapę `source_id` → nazwa w handlerze **dokładnie tym argumentem** — „mapa wprowadzałaby cache, którego nie da się odświeżyć bez restartu aplikacji, a źródła zmienia się przez `psql`". Ten sam zarzut stosuje się do crawlera, który cache'uje całą listę targetów na czas życia procesu, i spec go tam nie stawia.

**Kategoria:** Ambiguous

**Skutek:** historyjka 4 jest formalnie spełniona (restart kontenera to nie redeploy, `restart: always` w `docker-compose.yaml:50` to ułatwia), ale spec nigdzie tego nie mówi. Implementujący, który potraktuje historyjkę 4 dosłownie, doda odczyt targetów na początku każdego przebiegu — co jest sensowne, tanie (jedno zapytanie co 4-5 h) i **zmienia strukturę `OfferFetcher`**, bo lista targetów przestaje być polem ustawianym w konstruktorze.

**Rekomendacja:** jedno zdanie rozstrzygające: „lista targetów jest czytana raz przy starcie; zmiana `enabled` wymaga `docker compose restart crawler`" — albo odwrotnie.

---

### M7. `maxOffersPerPass = 100` systematycznie zagłodzi późniejsze kategorie

**Odniesienie:** spec §9.6, §8.1 (`ORDER BY s.slug, st.category, st.id`), §15.2.

**Dowód:** targety są przetwarzane sekwencyjnie w kolejności zwróconej przez `ListEnabledScrapeTargets`, czyli **deterministycznie alfabetycznie po kategorii**: `ai-ml`, `architect`, `c`, `c`, `go`, `go`, `php`. Transformer zeruje `scrapedInPass` przy zmianie numeru przebiegu, ale kolejność jest w każdym przebiegu ta sama.

Dla Bulldogjoba `approved-urls.md:52-53` podaje zmierzone liczby: `role,architect` → 50 ofert (limit strony), `role,ai` → 50 ofert. Przy limicie 3 stron to do ~300 ofert na sam `ai-ml` i `architect` — czyli **budżet 100 ofert wyczerpuje się na dwóch pierwszych kategoriach, w każdym przebiegu**. `go` (12 ofert), `php` (21) i `c` nie zostaną tknięte, dopóki `ai-ml` i `architect` się nie wyczerpią — co przy 100 ofertach na 4-5 h zajmie kilka dni.

Spec kwituje to zdaniem: „Pominięte targety zostaną odkryte ponownie w kolejnym przebiegu" (§9.6). Zostaną odkryte, ale **będą pomijane z tego samego powodu**, przebieg po przebiegu.

**Kategoria:** Incomplete

**Skutek:** przez pierwsze dni użytkownik nie zobaczy ani jednej oferty Go z Bulldogjoba — czyli z kategorii, która jest głównym motywem całego projektu (`project/vision.md`). To nie jest awaria, ale jest to zachowanie zaskakujące i niewyjaśnione.

**Rekomendacja:** odnotować w §15.2 jako świadomy dług, albo dodać rotację offsetu targetów między przebiegami (`Pass % len(targets)`) — dwie linie kodu.

---

## 4. Ustalenia niskiej wagi

| # | Ustalenie | Dowód | Kategoria |
|---|---|---|---|
| **L1** | `Makefile:71` nie istnieje — plik ma 58 wierszy. `DROP SCHEMA public CASCADE` jest w `Makefile:55`, `psql -f .../schema.sql` w `:56` | spec §7.4 i §15.2; błąd odziedziczony z `gap-analysis.md:187` — propagacja przez dwa artefakty | Incorrect (cytat) |
| **L2** | `docker-compose.yaml:48,51` → faktycznie `restart: always` jest w `:50`, `init: true` w `:54` | spec §7.2 i §11.1; błąd odziedziczony ze `scope-clarifications.md` („`init: true` ... w `docker-compose.yaml:51`") | Incorrect (cytat) |
| **L3** | „`grep source` w całym repo: 3 trafienia" — faktycznie **1** (`internal/category/category.go:16`, w komentarzu „single source of truth") | spec §11.2 | Incorrect (cytat) — teza jest mocniejsza, niż spec sądzi |
| **L4** | „Wcześniejsze «21» i «22» ... były **błędem arytmetycznym**" — nieprawda. `scope-clarifications.md:35` odnotowuje `bulldogjob skills,C` jako **zatwierdzone rozszerzenie zakresu** w Fazie 2 („jako 22. wiersz seedu"), a nie korektę rachunku. 22 było poprawną liczbą przed dodaniem URL-a | spec §6 (nota) i §16 poz. 1 vs `scope-clarifications.md:35`, `approved-urls.md:82` | Incorrect (narracja) |
| **L5** | `approved-urls.md` pozostaje wewnętrznie sprzeczny: tabela Bulldogjoba (`:48-54`) ma **5 wierszy**, `:59` nadal mówi o `skills,C`, że „nie mieścił się w zatwierdzonej liście — **wymaga osobnej zgody**", a `:80,82` mówią o 6 wierszach i zatwierdzeniu. Twierdzenie specyfikacji „artefakty zostały poprawione 2026-09-04" jest prawdziwe tylko dla podsumowania | `approved-urls.md:48-59` vs `:80-82` | Incomplete (artefakt wejściowy) |
| **L6** | `ListPossibilitiesByCategory` w §8.2 wybiera `p.id`, którego nikt nie konsumuje — `possibilityRow` (`scraped_handler.go:28-33`) nie ma pola `ID` i §10.4 go nie dodaje. Zapytanie zmienia się z `SELECT *` na jawną listę **właśnie po to**, by spełnić „Select Only Needed Columns" | spec §8.2 vs `scraped_handler.go:62-70` | Incorrect (drobiazg) |
| **L7** | `seed.sql` montowany do `docker-entrypoint-initdb.d` (§7.2) nie zostanie załadowany przez `make db-reset`, bo `Makefile:56` ładuje wyłącznie `schema.sql`. Po lokalnym resecie crawler wystartuje i wypisze „no enabled scrape targets, crawler has nothing to do" | `Makefile:54-56` vs spec §7.2 | Missing (lokalny dev) |

---

## 5. Luki nieujęte w powyższych — dokumentacja i weryfikacja

### D1. Dokumentacja projektu nie jest w zakresie, mimo że zadanie unieważnia jej treść

`.maister/docs/project/tech-stack.md:34` stwierdza wprost:

> „no ORM, no runtime reflection, **no migration framework needed** since the schema file doubles as the Postgres container's init script (`docker-entrypoint-initdb.d`)"

To zadanie wprowadza katalog `internal/database/migrations/`, nazwany plik migracji, drugi plik ładowany przez `docker-entrypoint-initdb.d` oraz trzy nowe tabele. `gap-analysis.md:572` rekomenduje aktualizację `tech-stack.md` w trzech punktach. Spec przejmuje z tej rekomendacji **wyłącznie** uwagę o `make sqlc-generate` (§8.3) i odsyła ją poza zakres.

`standards/global/conventions.md` → „Up-to-Date Documentation"; `CLAUDE.md` → „Standards Evolution". **Waga: średnia**, ale zaklasyfikowana tu, bo to poprawka jednoakapitowa, a nie praca projektowa.

**Rekomendacja:** dopisać aktualizację `tech-stack.md` do zakresu (jeden akapit) albo odnotować jako świadomy dług w §15.2.

### D2. Kryteria 12 i 13 są praktycznie nieweryfikowalne w normalnym przebiegu weryfikacji

Kryterium 12 wymaga potwierdzenia znacznikami czasu w logu, że odstęp między przebiegami listingu mieści się w `[4h, 5h)`. Kryterium 13 — że żaden pipeline nie przekracza 100 ofert na przebieg. Kryterium 6 — stanu bazy „po pełnym przebiegu crawlera".

Throttling jest zestawem stałych `const` bez zmiennych środowiskowych (świadomy dług, `scope-clarifications.md`), więc **nie ma żadnego sposobu skrócenia pętli na potrzeby weryfikacji**. Potwierdzenie kryterium 12 wymaga dziewięciu godzin obserwacji produkcji, kryterium 13 — pełnej doby, kryterium 6 — kilku dni.

Nie jest to zarzut wobec decyzji o `const` (użytkownik ją podjął świadomie), lecz wobec tego, że **spec nie łączy tego długu ze swoimi własnymi kryteriami akceptacji**. Weryfikator dostanie listę 24 punktów, z których 3-4 nie da się zamknąć w ramach fazy weryfikacji.

**Rekomendacja:** oznaczyć kryteria 6, 12, 13 i 14 jako „weryfikacja odroczona — obserwacja produkcji", z jawnym terminem, albo dopuścić jednorazowe tymczasowe skrócenie stałych na czas weryfikacji.

### D3. Fixture'y testowe mają zależność cykliczną z implementacją

§13.1 wymaga, by fixture'y HTML były „pozyskane dokładnie tą drogą, którą zapisuje etap 1". Etap 1 (`fetchOfferHTML`) potrzebuje działającego `ReadySelector()` z każdego parsera (kwestia otwarta 3), a dotarcie do strony oferty wymaga działającego kolektora (kwestia otwarta 2). Testy parserów — jedyne pokrycie tej zmiany — można więc napisać dopiero po zbudowaniu kolektorów i etapu 1.

Nie jest to wada projektu, ale **twarde ograniczenie kolejności dla planu implementacji**, którego spec nie wymienia. Dodatkowo: 20 plików realnego HTML-a SPA to prawdopodobnie kilka MB w repozytorium; spec nie wspomina o rozmiarze ani o tym, że strony ofert zawierają dane firm.

### D4. Punkt do zweryfikowania przy implementacji: nullability `s.name` po `LEFT JOIN`

§8.2 zakłada, że `sqlc` wygeneruje `pgtype.Text` dla `s.name AS source_name` po `LEFT JOIN`, a §10.4 buduje na tym `formatText(p.SourceName)` → `-`. Od tego zależy `component:source-null-fallback` i kryterium akceptacji 17. Wnioskowanie o nullability przy `LEFT JOIN` jest w `sqlc` funkcją stosunkowo świeżą; jeśli wygeneruje się `string`, cała ścieżka `-` wymaga przeprojektowania (`COALESCE(s.name, '')` w SQL-u i inny helper). Awaria będzie głośna (błąd kompilacji lub skanowania), więc waga jest niska — ale to pierwszy krok do wykonania w grupie zadań sqlc.

---

## 6. Ocena kwestii otwartych 2-6

| # | Kwestia | Da się odłożyć? | Uzasadnienie |
|---|---|---|---|
| **2** | Mechanika listingu NoFluffJobs i solid.jobs; kształt URL-a strony 2-3 Bulldogjoba | **Tak, z zastrzeżeniem** | Sam DOM i selektory — bez wątpienia. Ale jeśli Bulldogjob okaże się mieć „load more" zamiast paginacji z URL-em, stała `maxListingPages = 3` (§9.6) traci sens, a wraz z nią wiersz o pomijaniu świeżych ofert w §15.2. Zastrzeżenie: plan implementacji powinien mieć **jawne zadanie rozpoznania mechaniki przed** grupą kolektorów, a nie w jej środku |
| **3** | `ReadySelector()` per portal | **Tak, bez zastrzeżeń** | Czysta wiedza o DOM-ie. Kontrakt `fetchOfferHTML` (§9.3) jest zdefiniowany niezależnie od wartości selektora, a `htmlReadyTimeout` czyni brak trafienia nieblokującym. Jedyna kwestia otwarta, którą można odłożyć bez żadnych warunków |
| **4** | Czy podnieść `listingTimeout` (3 min) | **Nie w obecnej postaci** | Blokuje ją **M5**: dopóki nie wiadomo, kto otwiera kartę (fetcher czy kolektor), pytanie „czy 3 minuty wystarczą na 3 strony" nie ma jednoznacznego znaczenia — w wariancie B każda strona ma własne 3 minuty i problem nie istnieje. **Trzeba rozstrzygnąć własność karty na etapie planowania**; sama wartość liczbowa może zaczekać |
| **5** | Recykling Chromium co N ofert | **Tak** | Decyzja strojeniowa po pierwszym przebiegu. Warunek: `transformBatch` musi trzymać tworzenie przeglądarki w jednym miejscu (dziś `transformer.go:62-67`), żeby dołożenie recyklingu było zmianą lokalną. Warto to dopisać do §9.3 jako wymóg strukturalny — inaczej `defer closeBrowser()` rozejdzie się po pętli i późniejsze dołożenie recyklingu stanie się refaktorem |
| **6** | Rozmiar puli `pgxpool` | **Tak, ale plan musi to przewidzieć** | Sama wartość — tak. Ale spec zakłada milcząco, że to zmiana konfiguracyjna, a `internal/config/loader.go:55-60` buduje DSN z jedynym parametrem `sslmode=disable`: **`pool_max_conns` nie da się dziś przekazać bez zmiany kodu**. Jeśli odpowiedź brzmi „podnieść", dochodzi plik, którego nie ma w żadnym inwentarzu specyfikacji. Powiązane z **H5** — to ta sama pula, ten sam scenariusz wysycenia |

**Wniosek:** żadna z pięciu kwestii nie blokuje planowania w całości, ale **kwestia 4 blokuje częściowo** i domaga się rozstrzygnięcia własności karty przeglądarki (M5) przed napisaniem planu. Kwestia 6 wymaga jednego zdania w inwentarzu plików.

---

## 7. Zgodność ze standardami `.maister/docs/standards/`

| Standard | Ocena | Uwagi |
|---|---|---|
| `backend/migrations.md` | **Zgodny z odstępstwami, poprawnie odnotowanymi** | „Descriptive Names", „Small and Focused", „Version Control" (append-only) — spełnione. „Reversible" i brak ewidencji — świadome odstępstwa, uzasadnione skalą i idempotencją. **Ale** „Separate Schema and Data" jest zadeklarowane jako spełnione, a oba kroki `UPDATE` nie mają pliku ani miejsca wykonania (**H2**) |
| `backend/models.md` | **Zgodny** | Tabela w §5.2 to najlepiej udokumentowana część specyfikacji. Rezygnacja z osobnego indeksu na `scrape_targets.source_id` (pokryty przez `UNIQUE (source_id, url)`) jest technicznie poprawna i świadomie odchodzi od rekomendacji `gap-analysis.md:1.1`. Nullable `source_id` — uzasadnione istniejącymi danymi |
| `backend/queries.md` | **Zgodny z jednym odstępstwem** | „Parameterized", „Avoid N+1", „Index Strategic Columns" — spełnione. „Transactions" — świadomy dług. **Uwaga:** „Select Only Needed Columns" jest deklarowanym motywem zmiany `ListPossibilitiesByCategory`, a nowe zapytanie i tak wybiera niekonsumowane `p.id` (**L6**). „Query Timeouts" — spełnione, ale asymetrycznie (**H5**) |
| `global/minimal-implementation.md` | **Zgodny z jednym naruszeniem** | §11.3 („Świadomie **nie**budowane") to wzorowa sekcja. Naruszenie: proponowane dołożenie `chromedp.Flag("disable-dev-shm-usage", true)`, które duplikuje flagę już aktywną (**M2**) — dokładnie „Unused Code Is Debt" |
| `global/error-handling.md` | **Zgodny** | „Graceful Degradation" (nieznany slug → log + pominięcie), „Fail Fast" (asercja `current_schema()`, walidacja slugów), „Error Wrapping", „Resource Cleanup" — wszystko obecne. Wymóg logowania braku pola opcjonalnego w parserze to realna poprawa wobec dzisiejszego cichego `""` (`transformer.go:150-152`) |
| `global/coding-style.md` | **Zgodny** | Nazewnictwo plików `snake_case`, konstruktory `New<X>`, DRY dla stałej placeholdera i `absoluteURL`. Przemianowanie `frequency` → `listingInterval` uzasadnione „Descriptive Names" |
| `global/commenting.md` | **Zgodny** | Jawny zakaz komentarzy change-log („dodane w migracji 001", „było 3 s") — trafne, bo repo tej dyscypliny faktycznie przestrzega |
| `global/validation.md` | **Zgodny** | Walidacja wielowarstwowa: `CHECK`, `UNIQUE`, walidacja slugów przy starcie, allowlist portali literalnie w `main()` |
| `global/conventions.md` | **Zgodny z dwoma odstępstwami** | „Interface-Based DI", „Build What You Need", „Predictable Structure" — spełnione. Odstępstwo od „Environment Variables" (throttling jako `const`) jest zatwierdzone, ale jego konsekwencja dla weryfikowalności kryteriów nie została wyciągnięta (**D2**). **„Up-to-Date Documentation" nie jest spełnione** (**D1**) |
| `testing/test-writing.md` | **Zgodny z odnotowanym długiem** | Testy tabelaryczne, stdlib, stuby, nazwy jako zdania — wszystko zgodne z `transformer_test.go`. Brak testu integracyjnego odrzucony świadomie i uczciwie opisany jako „jedyne pokrycie tej zmiany". Nieujęte: cykliczna zależność fixture'ów (**D3**) |

---

## 8. Wykonalność 24 kryteriów akceptacji

| Kategoria | Kryteria | Ocena |
|---|---|---|
| Weryfikowalne mechanicznie, natychmiast | 1, 2, 3, 4, 5, 7, 8, 16, 17, 18, 19, 20, 21, 22, 23 | **15/24** — dobre. Kryterium 5 (`git diff` na `CreatePossibility`) jest szczególnie mocne: sprawdza dokładnie to, co jest objęte zakazem regresji |
| Weryfikowalne, ale wymagają obserwacji produkcji | 6, 9, 10, 11, 12, 13, 15 | **7/24** — kryteria 12 i 13 wymagają odpowiednio ~9 h i ~24 h obserwacji, bez możliwości skrócenia (**D2**) |
| **Wadliwe** | 14, 24 | **2/24** — kryterium 14 zwróci fałszywy alarm po backfillu (**H3**); kryterium 24 nie testuje zależności tranzytywnej (**M3**) |

Drobniejsze niejednoznaczności sformułowań:
- **kryterium 2** — „literał `public` w kontekście kwalifikacji obiektu" wymaga oceny człowieka; automatyczny grep na `*.sql` przechodzi (żaden plik SQL w repo nie zawiera `public`), ale `Makefile:55` zawiera `DROP SCHEMA public CASCADE`. Warto doprecyzować zakres.
- **kryterium 3** — `count(*) FROM scrape_targets = 23` jest prawdziwe wyłącznie **bezpośrednio po seedzie**. F1 zakłada, że użytkownik dodaje i wyłącza targety przez `psql`, więc kryterium ma krótki termin ważności.

---

## 9. Czego audyt świadomie nie zgłasza

Zgodnie z instrukcją audytu, poniższe zostały zweryfikowane wyłącznie pod kątem tego, **czy spec je rzetelnie odnotowuje** — i odnotowuje je wszystkie, z konsekwencjami:

| Dług | Gdzie odnotowany | Ocena rzetelności |
|---|---|---|
| Brak testu integracyjnego (`ON CONFLICT` bez pokrycia) | §12, §13.3, §15.2 | **Wzorowa** — spec trzy razy powtarza, że mechanizm objęty zakazem regresji nie ma żadnego pokrycia |
| Brak transakcji przy `possibilities` + `possibility_html` | §12, §15.2, kryterium 14 | Rzetelna, ale mechanizm wykrywania sierot jest wadliwy (**H3**) |
| `possibility_html` write-only bez retencji i bez wyłączenia z `pg_dump` | §3.2, §15.2 | **Rzetelna** — wskazuje konkretnie `deploy.sh:24-29` i rosnący czas każdego deployu |
| Brak `CRAWLER_*` dla throttlingu | §9.6, §12 | Rzetelna co do decyzji; niewyciągnięta konsekwencja dla weryfikacji (**D2**) |
| Brak frameworka migracji z ewidencją | §7.1, §12, §15.2 | **Rzetelna** — uzasadniona idempotencją każdego statementu |
| Deduplikacja semantyczna poza zakresem | §3.2, §2 historyjka 2, §13.2 | **Rzetelna** — a kolumna `Source` jest przedstawiona jako świadome wsparcie dla ręcznej deduplikacji, nie jako jej namiastka |

---

## 10. Rekomendacje w kolejności działania

**Przed fazą planowania (blokujące):**
1. **H1** — dopisać do §7.1 kształt wywołania `001_multi_portal_sources.sql` z `deploy.sh` i los istniejącego bloku heredoc (`deploy.sh:56-78`).
2. **H2** — przypisać kroki 1, 3 i 6 runbooka do konkretnego pliku lub jawnie oznaczyć jako operacje ręczne; poprawić kolejność względem `deploy.sh:32` (`compose down`).
3. **H3** — dodać guard czasowy do kryterium akceptacji 14.
4. **M3** — zamienić kryterium 24 na `go list -deps ./cmd/web | grep chromedp`.
5. **M5** — rozstrzygnąć w §9.1, czy `CollectOfferURLs` dostaje kontekst przeglądarki czy gotową kartę (odblokowuje kwestię otwartą 4).

**Przed fazą implementacji (istotne):**
6. **H4** — odnotować nieodwracalność wierszy `(unparsed)` w §15.2, z procedurą naprawczą.
7. **H5** — rozstrzygnąć `knownOffersTimeout` (wypełnić pustą komórkę „Powód" w §9.6).
8. **M1** — poprawić §8.3 na trzy miejsca łamania (`routing_test.go:118`).
9. **M2** — usunąć z zakresu dołożenie `disable-dev-shm-usage` i przeformułować uzasadnienie `mem_limit` w §7.2 i §9.5.
10. **M4** — przypisać logikę retry (`maxFetchAttempts`, `fetchRetryBaseDelay`) i zamienić `fetcher.go:103` `time.Sleep` na `pause(ctx, ...)`.

**Higiena dokumentu (drobne):**
11. **L1-L3** — poprawić trzy cytaty: `Makefile:71` → `:55`, `docker-compose.yaml:48,51` → `:50,54`, „3 trafienia" → 1.
12. **L4-L5** — poprawić narrację o liczbie 23 (rozszerzenie zakresu, nie błąd arytmetyczny) i doprowadzić `approved-urls.md:48-59` do zgodności z `:80-82`.
13. **M6, M7, L6, L7, D1** — po jednym zdaniu każde.

---

## Podsumowanie

Specyfikacja jest solidna i w większości dokładna — na tyle, że jej błędy trzeba było szukać w rzeczach, których w niej **nie ma** (retry, `routing_test.go`, mechanizm dostarczenia migracji), a nie w tym, co napisała. Jakość cytowania kodu jest wyższa niż w artefaktach poprzedzających: spec poprawił dwa błędne odniesienia odziedziczone z `technical-clarifications.md`, choć jednocześnie propagował dwa inne z `gap-analysis.md` i `scope-clarifications.md`.

Najpoważniejszy pojedynczy problem to nie żaden z błędów technicznych, lecz **brak mostu między runbookiem deployu (§7.3) a rzeczywistym `scripts/deploy.sh`** — osiem starannie ponumerowanych kroków, z których trzy nie mają gdzie się wykonać. Przy `risk_level: high` i `compatibility_requirements: strict` to jest miejsce, w którym zadanie może się realnie wywrócić na produkcji.

**Werdykt: `pass-with-concerns`** — dopuścić do planowania po naprawieniu pięciu punktów blokujących (H1, H2, H3, M3, M5).
