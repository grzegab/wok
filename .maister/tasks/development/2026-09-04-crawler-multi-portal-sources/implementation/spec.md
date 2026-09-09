# Specyfikacja: crawler wieloportalowy (NoFluffJobs, Bulldogjob, solid.jobs)

**Zadanie:** `2026-09-04-crawler-multi-portal-sources`
**Data:** 2026-09-04
**Ryzyko:** wysokie · **Wpływ architektoniczny:** wysoki · **Typ zmiany:** `modificative` · **Kompatybilność:** `strict`
**Charakterystyki:** `modifies_existing_code`, `creates_new_entities`, `involves_data_operations`, `ui_heavy`

**Wejścia wiążące:** `implementation/requirements.md`, `analysis/technical-clarifications.md`, `analysis/clarifications.md`, `analysis/scope-clarifications.md`, `analysis/approved-urls.md`, `analysis/gap-analysis.md`, `analysis/codebase-analysis.md`, `analysis/design-context/`.
Ta specyfikacja **rozwija** te dokumenty i nie podejmuje decyzji na nowo. Wszystko, czego nie dało się z nich wyprowadzić, trafiło do sekcji [16. Kwestie otwarte](#16-kwestie-otwarte).

---

## 1. Cel

Rozszerzyć crawlera z jednego portalu (justjoin.it) na cztery (justjoin.it, nofluffjobs.com, bulldogjob.pl, solid.jobs), przenosząc rejestr źródeł i URL-e listingów z kodu Go do bazy danych, zapisując surowy HTML każdej oferty i spowalniając throttling do 2-3 min na ofertę oraz 4-5 h na listing — **bez najmniejszej zmiany w zachowaniu deduplikacji po URL i bez przerwy w działaniu widoków `/scr4p3d/{category}` i `/st4ts/{category}`**.

---

## 2. Historyjki użytkownika

Aplikacja ma jednego użytkownika — autora (`project/vision.md`).

1. Jako użytkownik chcę widzieć oferty z czterech portali w jednej liście, żeby nie odwiedzać każdego portalu z osobna.
2. Jako użytkownik chcę widzieć, **z którego portalu** pochodzi każda oferta, żeby rozpoznać tę samą ofertę powtórzoną w kilku wierszach (deduplikacja semantyczna jest poza zakresem i robię ją ręcznie).
3. Jako użytkownik chcę mieć zapisany surowy HTML każdej oferty, żeby móc później — poza aplikacją, z pomocą LLM — wydobyć z niego dane, których dzisiejszy parser nie wyciąga.
4. Jako administrator (ten sam człowiek) chcę włączać i wyłączać pojedyncze pary portal × kategoria przez `psql`, bez rekompilacji i redeploya binarki.
5. Jako administrator chcę, żeby crawler chodził wolno i nieregularnie, żeby nie dostać bana na żadnym z czterech portali.

---

## 3. Zakres

### 3.1 W zakresie

| # | Obszar | Skrót |
|---|---|---|
| F1 | Rejestr źródeł w bazie | `sources`, `scrape_targets`, `possibilities.source_id` |
| F2 | Odkrywanie ofert per portal | interfejs `LinkCollector` + 4 implementacje |
| F3 | Parsowanie ofert per portal | interfejs `OfferParser` + 4 implementacje, seam pobranie/parsowanie |
| F4 | Zapis surowego HTML | `possibility_html`, dwa `INSERT`-y, timeout 60 s |
| F5 | Polityka nieudanego parsowania | wiersz z placeholderem + HTML |
| F6 | Throttling | 2-3 min / oferta, 4-5 h / listing, limit 100 ofert na przebieg |
| F7 | Równoległość per portal | 4 niezależne pipeline'y |
| F8 | Kolumna `Source` w UI | `scraped/list.gohtml` |
| F9 | Dostarczenie zmian schematu | migracja + `seed.sql` |

### 3.2 Poza zakresem (jawnie)

- Deduplikacja semantyczna między portalami (użytkownik robi ją ręcznie, poza aplikacją).
- Normalizacja nazw technologii w kolumnie `tech`.
- Filtrowanie po źródle i statystyki per źródło.
- CRUD źródeł w UI, podgląd surowego HTML w UI, trasa `/scr4p3d/{category}/{id}/html`.
- Retencja, kompresja i wyłączanie `possibility_html` z `pg_dump`.
- Test integracyjny przeciw realnemu Postgresowi.
- Zmienne środowiskowe `CRAWLER_*` dla throttlingu.
- Framework migracji z tabelą `schema_migrations`.
- `CREATE SCHEMA wok` w kontroli wersji i naprawa `make db-reset` pod schemat `wok`.
- User-Agent, rotacja proxy, obsługa CAPTCHA.
- Paginacja / wirtualizacja listy ofert w UI (lista urośnie ~4×, `table-responsive` wystarcza).
- Zmiana `possibilities.category` z `Name` na slug (osobne zadanie, patrz `scope-clarifications.md` → `category-key-mismatch`).

---

## 4. Kroki wymagane PRZED napisaniem DDL

Dwa zapytania na **produkcji**. Obie odpowiedzi zmieniają treść pliku migracji, więc są twardą bramką.

```sql
-- 1. Postać istniejących URL-i (absolutne czy relatywne)
SELECT url FROM possibilities LIMIT 5;
SELECT count(*) FROM possibilities WHERE url NOT LIKE 'http%';

-- 2. Potwierdzenie, że category trzyma Name, a nie slug
SELECT DISTINCT category FROM possibilities;
```

**Interpretacja zapytania 1.** Wymuszenie URL-a absolutnego (decyzja `url-canonicalization`) jest **nieodwracalne**: jeśli istniejące wiersze justjoin.it są relatywne, a nowy kod zapisuje absolutne, powstanie komplet duplikatów wszystkich dotychczasowych ofert i `ExistingPossibilityUrls` przestanie je rozpoznawać. Jeśli `count(*) > 0`, do migracji **dochodzi jednorazowy krok danych**:

```sql
-- Krok danych. Wykonać TYLKO gdy poprzedzające zapytanie zwróciło > 0.
UPDATE possibilities
SET url = 'https://justjoin.it' || url
WHERE url LIKE '/%';
```

**Interpretacja zapytania 2.** Oczekiwany wynik to dokładnie `Go`, `PHP`, `Software Architect`, `AI/ML`, `C/C++`. Każdy inny wynik unieważnia założenie z sekcji [9.3](#93-przepływ-danych-i-typy) i wymaga powrotu do decyzji `category-key-mismatch`.

---

## 5. Model danych

Cztery zmiany schematu. Całość opiera się na **niekwalifikowanych nazwach obiektów**, które rozwiązują się przez `search_path` (na VPS: schemat `wok`); każdy guard w `DO $$` porównuje się z `current_schema()`. **Nigdzie nie pada literał `public`.**

### 5.1 DDL

```sql
-- 1. Rejestr portali.
CREATE TABLE sources (
    id         BIGSERIAL PRIMARY KEY,
    slug       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 2. URL-e listingów: portal x kategoria x URL. Wiele URL-i na parę portal x kategoria
--    jest dozwolone (NoFluffJobs: /pl/go + /pl/golang, /pl/c + /pl/c++;
--    Bulldogjob: skills,C + skills,C++), dlatego klucz unikalności NIE obejmuje kategorii.
CREATE TABLE scrape_targets (
    id         BIGSERIAL PRIMARY KEY,
    source_id  BIGINT NOT NULL REFERENCES sources (id) ON DELETE CASCADE,
    category   TEXT NOT NULL CHECK (category <> ''),
    url        TEXT NOT NULL,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_id, url)
);

-- 3. Powiązanie oferty ze źródłem. NULLABLE - wiersze sprzed backfillu nie mają źródła.
--    Brak klauzuli ON DELETE jest celowy (NO ACTION): usunięcie portalu, który ma oferty,
--    ma się wywalić głośno, a nie po cichu wyzerować lub skasować historię.
ALTER TABLE possibilities ADD COLUMN source_id BIGINT REFERENCES sources (id);

CREATE INDEX IF NOT EXISTS possibilities_source_id_idx ON possibilities (source_id);

-- 4. Surowy HTML, relacja 1:1. possibility_id jako PK daje jednocześnie klucz obcy,
--    unikalność i darmową idempotencję zapisu przy ON CONFLICT - potrzebną, bo
--    CreatePossibility jest upsertem i przy powtórnym scrapie zwróci istniejące id.
CREATE TABLE possibility_html (
    possibility_id BIGINT PRIMARY KEY REFERENCES possibilities (id) ON DELETE CASCADE,
    html           TEXT NOT NULL,
    fetched_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 5.2 Uzasadnienia wobec `standards/backend/models.md`

| Reguła standardu | Realizacja |
|---|---|
| Clear Naming | tabele w liczbie mnogiej (`sources`, `scrape_targets`); `possibility_html` w liczbie pojedynczej, bo to relacja 1:1 rozszerzająca jeden wiersz, nie kolekcja |
| Timestamps | `created_at` w `sources` i `scrape_targets`, `fetched_at` w `possibility_html`. **Brak `updated_at`** — żadna z tych tabel nie jest edytowana z aplikacji (poza `enabled` przez `psql`), a dodanie kolumny bez pisarza łamie `minimal-implementation.md` |
| Database Constraints | `NOT NULL` wszędzie, gdzie dane istniejące na to pozwalają; `UNIQUE (slug)`, `UNIQUE (source_id, url)`, `CHECK (category <> '')` |
| Appropriate Types | `BIGINT`/`BIGSERIAL` spójnie z `possibilities.id`; `TEXT` dla HTML (Postgres kompresuje TOAST-em — **kompresja aplikacyjna zbędna**); `TIMESTAMPTZ` spójnie z resztą schematu |
| Index Foreign Keys | `possibilities_source_id_idx` utworzony jawnie. Dla `scrape_targets.source_id` **osobny indeks nie powstaje**: `UNIQUE (source_id, url)` tworzy indeks z `source_id` na pierwszej pozycji i w pełni obsługuje wyszukiwanie po kluczu obcym. Dołożenie drugiego byłoby martwym kodem (`minimal-implementation.md`) |
| Clear Relationships | `possibility_html` → `ON DELETE CASCADE` (bez niego kasowanie oferty blokuje się na FK); `scrape_targets` → `ON DELETE CASCADE` (usunięcie portalu usuwa jego konfigurację); `possibilities.source_id` → `NO ACTION` (świadomie, patrz komentarz w DDL) |
| Practical Normalization | `scrape_targets.category` to **wolny `TEXT` bez FK** — kategorie zostają w Go (decyzja `category-source-of-truth`). Ochroną jest `CHECK` plus walidacja slugów przy starcie crawlera (patrz [9.4](#94-wiązanie-pipelineów-i-graceful-shutdown)) |

### 5.3 Czego NIE wolno ruszyć

Zakaz regresji dedupu (`requirements.md`, wymagania niefunkcjonalne) oznacza konkretnie:

- `possibilities.url TEXT NOT NULL UNIQUE` — bez zmian; **w szczególności nie zamieniać na `UNIQUE (source_id, url)`**.
- `CreatePossibility ... ON CONFLICT (url)` — **cel konfliktu bez zmian**. Klauzula `DO UPDATE SET` zostaje rozszerzona (rozstrzygnięcie H4 z audytu, 2026-09-04): poza dotychczasowym `posted_at = COALESCE(EXCLUDED.posted_at, possibilities.posted_at)` nadpisuje `job_title`, `company_name` i pozostałe pola treści **tylko wtedy, gdy wiersz w bazie jest placeholderem**:
  ```sql
  ON CONFLICT (url) DO UPDATE SET
      posted_at    = COALESCE(EXCLUDED.posted_at, possibilities.posted_at),
      job_title    = CASE WHEN possibilities.job_title = '(unparsed)' THEN EXCLUDED.job_title    ELSE possibilities.job_title    END,
      company_name = CASE WHEN possibilities.job_title = '(unparsed)' THEN EXCLUDED.company_name ELSE possibilities.company_name END,
      location     = CASE WHEN possibilities.job_title = '(unparsed)' THEN EXCLUDED.location     ELSE possibilities.location     END,
      is_remote    = CASE WHEN possibilities.job_title = '(unparsed)' THEN EXCLUDED.is_remote    ELSE possibilities.is_remote    END,
      tech         = CASE WHEN possibilities.job_title = '(unparsed)' THEN EXCLUDED.tech         ELSE possibilities.tech         END,
      description  = CASE WHEN possibilities.job_title = '(unparsed)' THEN EXCLUDED.description  ELSE possibilities.description  END
  ```
  Dla wiersza poprawnie sparsowanego zachowanie jest **identyczne jak dziś** — zmienia się wyłącznie `posted_at`. Bez tego wiersze `(unparsed)` byłyby nieodwracalne: `unseen` odsiewa je jako znane, a stara klauzula nie dotykała treści.
- `source_id` dochodzi do listy `INSERT`, ale **nie** do `DO UPDATE SET` — źródło oferty nie zmienia się w czasie.
- `unseen` (`transformer.go:100`) **pomija wiersze z placeholderem**, żeby poprawiony parser mógł je nadpisać przy kolejnym przebiegu. Zapytanie `ExistingPossibilityUrls` zwraca wtedy tylko URL-e wierszy poprawnie sparsowanych.
- `ExistingPossibilityUrls` — **zmienia się** (rozstrzygnięcie R1, 2026-09-04): dochodzi warunek `AND job_title <> '(unparsed)'`, żeby wiersze z placeholderem nie liczyły się jako znane i poprawiony parser mógł je nadpisać. Go-owa funkcja `unseen` (`transformer.go:100`) widzi wyłącznie URL-e i nie zna `job_title`, więc bez zmiany po stronie SQL-a nie da się tego osiągnąć.
  ```sql
  -- name: ExistingPossibilityUrls :many
  SELECT url FROM possibilities WHERE url = ANY(@urls::text[]) AND job_title <> '(unparsed)';
  ```
  **Dla wierszy poprawnie sparsowanych zachowanie jest identyczne jak dziś.** Jest to jedyne odstępstwo od pierwotnego polecenia „dedup po URL nietknięty", świadomie zatwierdzone, bo bez niego wiersze `(unparsed)` byłyby nieodwracalne.
- `possibilities_category_created_at_idx` — bez zmian.

---

## 6. Zawartość `seed.sql`

Plik `internal/database/seed.sql`. Wyłącznie dane, zero DDL (`migrations.md` → „Separate Schema and Data"). W pełni idempotentny — uruchamiany zarówno przez `docker-entrypoint-initdb.d` na świeżej bazie, jak i przez `scripts/deploy.sh` na żywej.

```sql
INSERT INTO sources (slug, name) VALUES
    ('justjoin',    'justjoin.it'),
    ('nofluffjobs', 'nofluffjobs.com'),
    ('bulldogjob',  'bulldogjob.pl'),
    ('solidjobs',   'solid.jobs')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO scrape_targets (source_id, category, url)
SELECT s.id, v.category, v.url
FROM (VALUES
    -- justjoin.it (5) - stan istniejący, przeniesiony z internal/category/category.go
    ('justjoin',    'go',        'https://justjoin.it/job-offers/all-locations/go?orderBy=DESC&sortBy=newest'),
    ('justjoin',    'php',       'https://justjoin.it/job-offers/all-locations/php?orderBy=DESC&sortBy=newest'),
    ('justjoin',    'architect', 'https://justjoin.it/job-offers/all-locations/architecture?orderBy=DESC&sortBy=newest'),
    ('justjoin',    'ai-ml',     'https://justjoin.it/job-offers/all-locations/ai?orderBy=DESC&sortBy=newest'),
    ('justjoin',    'c',         'https://justjoin.it/job-offers/all-locations/c?orderBy=DESC&sortBy=newest'),

    -- nofluffjobs.com (7) - ?sort=newest zweryfikowany, realnie zmienia kolejność
    ('nofluffjobs', 'go',        'https://nofluffjobs.com/pl/go?sort=newest'),
    ('nofluffjobs', 'go',        'https://nofluffjobs.com/pl/golang?sort=newest'),
    ('nofluffjobs', 'php',       'https://nofluffjobs.com/pl/php?sort=newest'),
    ('nofluffjobs', 'architect', 'https://nofluffjobs.com/pl/architecture?sort=newest'),
    ('nofluffjobs', 'ai-ml',     'https://nofluffjobs.com/pl/artificial-intelligence?sort=newest'),
    ('nofluffjobs', 'c',         'https://nofluffjobs.com/pl/c?sort=newest'),
    -- Literalne '+' w sciezce jest poprawne i zweryfikowane. NIE kodowac jako %2B%2B.
    ('nofluffjobs', 'c',         'https://nofluffjobs.com/pl/c++?sort=newest'),

    -- bulldogjob.pl (6) - dla architect i ai-ml filtr roli, nie umiejetnosci
    ('bulldogjob',  'go',        'https://bulldogjob.pl/companies/jobs/s/skills,Go'),
    ('bulldogjob',  'php',       'https://bulldogjob.pl/companies/jobs/s/skills,PHP'),
    ('bulldogjob',  'architect', 'https://bulldogjob.pl/companies/jobs/s/role,architect'),
    ('bulldogjob',  'ai-ml',     'https://bulldogjob.pl/companies/jobs/s/role,ai'),
    ('bulldogjob',  'c',         'https://bulldogjob.pl/companies/jobs/s/skills,C++'),
    ('bulldogjob',  'c',         'https://bulldogjob.pl/companies/jobs/s/skills,C'),

    -- solid.jobs (5) - /praca/c-cpp-developer pokrywa C i C++ jedna strona
    ('solidjobs',   'go',        'https://solid.jobs/praca/golang-developer'),
    ('solidjobs',   'php',       'https://solid.jobs/praca/php-developer'),
    ('solidjobs',   'architect', 'https://solid.jobs/praca/architekt'),
    ('solidjobs',   'ai-ml',     'https://solid.jobs/praca/data-science'),
    ('solidjobs',   'c',         'https://solid.jobs/praca/c-cpp-developer')
) AS v (source_slug, category, url)
JOIN sources s ON s.slug = v.source_slug
ON CONFLICT (source_id, url) DO NOTHING;
```

> **Liczba wierszy: 23 — potwierdzone.** Wyliczenie zatwierdzonych URL-i daje 23 (justjoin.it 5 + nofluffjobs.com 7 + bulldogjob.pl 6 + solid.jobs 5). Wcześniejsze „21" i „22" w `approved-urls.md`, `scope-clarifications.md` i `requirements.md` były błędem arytmetycznym — artefakty zostały poprawione 2026-09-04. Źródłem prawdy jest wyliczona lista URL-i; każdy z nich zweryfikowany przeglądarką i osobno zatwierdzony.

**Zachowanie `ai-ml` na solid.jobs:** `/praca/data-science` to najbliższy tematycznie landing; świadomie akceptujemy wpadające tam oferty czysto danowe (Data Engineer, BI).

---

## 7. Strategia migracji i deployu

### 7.1 Pliki

| Plik | Rola | Uruchamiany przez |
|---|---|---|
| `internal/database/schema.sql` | pełny schemat docelowy | `docker-entrypoint-initdb.d` (świeży wolumen) **oraz** wejście dla `sqlc` |
| `internal/database/migrations/001_multi_portal_sources.sql` | idempotentne DDL doprowadzające istniejącą bazę do `schema.sql` | `scripts/deploy.sh` |
| `internal/database/seed.sql` | 23 wiersze `scrape_targets` + 4 wiersze `sources` | oba wejścia |
| `internal/database/data/001_normalize_urls.sql` | warunkowa normalizacja istniejących URL-i do postaci absolutnej (krok 3 runbooka) | `scripts/deploy.sh` |
| `internal/database/data/002_backfill_source_id.sql` | backfill `source_id` dla historycznych ofert justjoin.it (krok 6 runbooka) | `scripts/deploy.sh` |

**Mechanizm dostarczenia (rozstrzygnięcie H1 z audytu, 2026-09-04).** Katalog `internal/database/migrations/` **nie jest montowany** do kontenera Postgresa — `docker-compose.yaml` montuje wyłącznie `schema.sql` (i po tej zmianie `seed.sql`) do `docker-entrypoint-initdb.d`. `psql -f` wykonywane wewnątrz kontenera tych plików nie znajdzie. Pliki są więc czytane **na hoście** i podawane na standardowe wejście, dokładnie tak, jak `deploy.sh:55` robi to dziś z heredokiem:

```bash
$DOCKER compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" "$POSTGRES_DB"' \
    < internal/database/migrations/001_multi_portal_sources.sql
```

Zero nowej infrastruktury, zero zmian w montowaniach dla ścieżki migracji.

**Rozdzielenie schematu od danych (rozstrzygnięcie H2 z audytu).** Kroki danych z runbooka (`UPDATE`-y) mieszkają w osobnych plikach `internal/database/data/*.sql`, podawanych tym samym potokiem. Dzięki temu reguła „Separate Schema and Data" z `migrations.md`, którą [12. Zgodność ze standardami](#12-zgodność-ze-standardami) deklaruje jako spełnioną, jest faktycznie spełniona — plik migracji zawiera wyłącznie DDL, a `deploy.sh` pozostaje nieinteraktywny (`set -euo pipefail`).

`schema.sql` i plik migracji opisują ten sam stan docelowy dwiema drogami — to istniejąca konwencja repozytorium (`deploy.sh` już dziś ma blok „Applying schema updates") i świadomie utrzymana duplikacja. Rozjazd między nimi jest ryzykiem, więc jedno z kryteriów akceptacji sprawdza je względem siebie.

**Nazwa `001_multi_portal_sources.sql`** realizuje „Descriptive Names" z `migrations.md`. Plik jest **append-only**: po wdrożeniu nie wolno go edytować; kolejne zmiany dostają `002_...`. Bez tabeli `schema_migrations` — świadome odstępstwo, uzasadnione skalą projektu (`minimal-implementation.md`) i tym, że każdy statement jest idempotentny.

**Rollback:** brak automatyczny (odstępstwo od „Reversible"). Ścieżką odwrotu jest `pg_dump` robiony przez `deploy.sh:24-29` przed każdym wdrożeniem. Do odnotowania w [15. Ryzyka](#15-ryzyka-i-świadomie-przyjęte-długi-techniczne).

### 7.2 Zmiany w `docker-compose.yaml`

```yaml
  postgres:
    volumes:
      - ./internal/database/schema.sql:/docker-entrypoint-initdb.d/schema.sql:ro
      - ./internal/database/seed.sql:/docker-entrypoint-initdb.d/seed.sql:ro   # NOWE
```

`docker-entrypoint-initdb.d` wykonuje pliki w porządku alfabetycznym, więc `schema.sql` idzie przed `seed.sql` — kolejność jest poprawna bez dodatkowych prefiksów.

```yaml
  crawler:
    mem_limit: 4g        # NOWE - 8 jednoczesnych instancji Chromium w szczycie
```

`init: true` jest już ustawione (`docker-compose.yaml:54`) i przy 8 instancjach Chromium jest tym bardziej potrzebne.

**Korekta po audycie (2026-09-04): `disable-dev-shm-usage` NIE jest dodawane.** Flaga jest już aktywna — `chromedp` ma ją w `DefaultExecAllocatorOptions` (`chromedp@v0.16.0/allocate.go:69`), do których `newBrowser` (`internal/justjoin/browser.go:21-22`) jedynie dokleja własne opcje. Dopisanie jej byłoby martwym kodem, a ryzyko „domyślne 64 MB `/dev/shm`", na które powoływała się wcześniejsza analiza, **nie istnieje**. `shm_size` również nie jest potrzebne. Z decyzji o zasobach Chromium zostaje wyłącznie `mem_limit: 4g`.

### 7.3 Kolejność kroków przy deployu na żywą bazę

Kolejność jest istotna: krok 3 musi poprzedzać krok 6, bo backfill dopasowuje wiersze po prefiksie URL-a.

1. **Weryfikacja przed wdrożeniem** — zapytania z sekcji [4](#4-kroki-wymagane-przed-napisaniem-ddl). Wynik decyduje o obecności kroku 3.
2. `deploy.sh` → `make db-backup` (już istnieje, `deploy.sh:24-29`).
3. **Krok danych, warunkowy:** normalizacja istniejących URL-i do postaci absolutnej — plik `internal/database/data/001_normalize_urls.sql`, podawany potokiem z hosta (patrz [7.1](#71-pliki)). Wykonywany **tylko** gdy weryfikacja z kroku 1 wykazała URL-e relatywne; w `deploy.sh` bramkowany warunkiem, nie interaktywnym pytaniem.
4. **Asercja schematu** — nowy blok w `deploy.sh`, **przed** DDL:
   ```sql
   DO $$
   BEGIN
       IF current_schema() IS NULL THEN
           RAISE EXCEPTION 'no current schema on search_path';
       END IF;
       RAISE NOTICE 'applying DDL to schema %', current_schema();
   END $$;
   ```
   Dziś to, że DDL trafia do schematu `wok`, jest niezweryfikowanym założeniem opartym na ręcznej konfiguracji VPS-a. Wnosimy trzy tabele, więc asercja przestaje być opcjonalna.
5. **DDL** — `001_multi_portal_sources.sql`.
6. **Krok danych: backfill źródła** dla historycznych ofert — plik `internal/database/data/002_backfill_source_id.sql`, podawany tym samym potokiem. Wykonuje się w tym wdrożeniu (rozstrzygnięcie z 2026-09-04).
   ```sql
   UPDATE possibilities
   SET source_id = (SELECT id FROM sources WHERE slug = 'justjoin')
   WHERE source_id IS NULL
     AND url LIKE 'https://justjoin.it/%';
   ```
   Guard `LIKE` czyni krok samoweryfikującym: wiersze o innej postaci URL-a zostają `NULL` i wyrenderują się w UI jako `-`. Krok jest idempotentny.
7. **Seed** — `seed.sql`.
8. `docker compose up -d`.

**Stan przejściowy między krokiem 5 a 6 jest normalnym stanem produkcyjnym, nie przypadkiem brzegowym:** cała historyczna lista justjoin.it pokazuje `-` w kolumnie `Source`. UI musi to obsłużyć bezbłędnie (patrz [10](#10-projekt-wizualny), `component:source-null-fallback`).

### 7.4 Ostrzeżenie: `make db-reset`

`Makefile:55` wykonuje `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`, po czym ładuje `schema.sql`. Na VPS-ie, gdzie tabele żyją w schemacie `wok`, pierwsze polecenie nic nie usuwa, a drugie trafia w `wok` i wywala się na `relation "possibilities" already exists`. **Cel `db-reset` jest już dziś niebezpieczny na produkcji i po dodaniu trzech tabel będzie niebezpieczniejszy.** Naprawa jest poza zakresem tego zadania; wymóg to odnotowanie tego w [15. Ryzyka](#15-ryzyka-i-świadomie-przyjęte-długi-techniczne) i **nieużywanie** `db-reset` na VPS.

---

## 8. Warstwa `sqlc`

### 8.1 Nowy plik `internal/database/queries/sources.sql`

```sql
-- name: ListEnabledScrapeTargets :many
SELECT
    st.id,
    st.source_id,
    s.slug AS source_slug,
    s.name AS source_name,
    st.category,
    st.url
FROM scrape_targets st
JOIN sources s ON s.id = st.source_id
WHERE st.enabled
ORDER BY s.slug, st.category, st.id;
```

Jedno zapytanie przy starcie crawlera zamiast N zapytań per portal (`queries.md` → „Avoid N+1"). Jawna lista kolumn, bez `SELECT *` („Select Only Needed Columns").

### 8.2 Zmiany w `internal/database/queries/possibilities.sql`

```sql
-- name: CreatePossibility :one
INSERT INTO possibilities (
category, url, company_name, job_title, location, is_remote, tech, description, posted_at, source_id
) VALUES (
$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
) ON CONFLICT (url) DO UPDATE
SET posted_at = COALESCE(EXCLUDED.posted_at, possibilities.posted_at)
RETURNING *;

-- name: CreatePossibilityHtml :exec
INSERT INTO possibility_html (possibility_id, html)
VALUES ($1, $2)
ON CONFLICT (possibility_id) DO UPDATE
SET html = EXCLUDED.html, fetched_at = now();

-- name: ListPossibilitiesByCategory :many
SELECT
    p.id,
    p.url,
    p.job_title,
    p.tech,
    p.posted_at,
    s.name AS source_name
FROM possibilities p
LEFT JOIN sources s ON s.id = p.source_id
WHERE p.category = $1
ORDER BY p.created_at DESC, p.id DESC;
```

Uwagi krytyczne:

- `CreatePossibility` — dochodzi **wyłącznie** `source_id` jako `$10`. Klauzula `ON CONFLICT (url) DO UPDATE SET posted_at = ...` jest przepisana **znak w znak**. `source_id` celowo **nie** trafia do `DO UPDATE SET`: oferta widziana najpierw przez target A zachowuje źródło A. Przy absolutnych URL-ach kolizja między portalami nie zachodzi; kolizja między dwoma targetami tego samego portalu (NoFluffJobs `/pl/go` i `/pl/golang`) zachodzi realnie i daje ten sam `source_id`, co jest poprawne.
- `ListPossibilitiesByCategory` przechodzi z `SELECT *` na jawną listę kolumn — to wymóg `queries.md`, a nie kosmetyka: po dodaniu `source_id` `SELECT *` nadal by działało, ale jawna lista jest warunkiem tego, żeby przyszłe kolumny nie wpadały do handlera bez decyzji. **Zwracany typ zmienia się z `[]database.Possibility` na `[]database.ListPossibilitiesByCategoryRow`.**
- **Indeks pozostaje wykorzystany.** `WHERE p.category = $1 ORDER BY p.created_at DESC, p.id DESC` nadal pokrywa się z `possibilities_category_created_at_idx (category, created_at DESC, id DESC)`; `LEFT JOIN sources` po kluczu głównym czterowierszowej tabeli nie zmienia planu w istotny sposób.
- `s.name` jest nullable po `LEFT JOIN`, więc `sqlc` wygeneruje `pgtype.Text` — to jest właśnie nośnik stanu „wiersz sprzed backfillu".
- `GetPossibility`, `ExistingPossibilityUrls` i `StatsPossibilityByCategory` — **bez zmian**.

### 8.3 Regeneracja i wpływ na `Querier`

Regeneracja: `make sqlc` (nie `make sqlc-generate` — `project/tech-stack.md` podaje nieistniejący target; poprawka dokumentacji jest poza zakresem, ale warta zgłoszenia).

Po regeneracji `internal/database/generated/querier.go`:

- **przybywa:** `CreatePossibilityHtml(ctx, arg CreatePossibilityHtmlParams) error`, `ListEnabledScrapeTargets(ctx) ([]ListEnabledScrapeTargetsRow, error)`;
- **zmienia sygnaturę:** `ListPossibilitiesByCategory(ctx, category string) ([]ListPossibilitiesByCategoryRow, error)`;
- **przybywa model:** `Source`, `ScrapeTarget`, `PossibilityHtml`; `Possibility` dostaje pole `SourceID pgtype.Int8`.

Konsumenci deklarują wąskie interfejsy, więc łamią się **trzy** miejsca (korekta po audycie, 2026-09-04):

- `internal/handlers/scraped_handler.go:14-16` — `PossibilityStore.ListPossibilitiesByCategory` musi zmienić typ zwracany.
- `internal/justjoin/persist.go:11-13` — `OfferStore` dostaje drugą metodę.
- `internal/router/routing_test.go:118` — stub implementujący sklep w teście. **`go build` tego nie wykryje** — złamanie ujawnia dopiero `go test ./...`, więc weryfikacja po regeneracji `sqlc` musi obejmować budowanie testów, nie tylko binarek.

`var _ Querier = (*Queries)(nil)` nadal się kompiluje. Warstwa web nie wymaga innych zmian.

---

## 9. Architektura crawlera

### 9.1 Dwa wąskie interfejsy

Zgodnie z `technical-clarifications.md` p. 3: interfejs deklarowany **u konsumenta**, wyłącznie z metodami, których ten konsument używa — idiom obecny w całym repo (`knownOfferStore` w `transformer.go:28`, `OfferStore` w `persist.go:11`, `IpRepository`/`SessionRepository` w `router/routing.go:50-58`).

**W `internal/justjoin/fetcher.go`:**

```go
// LinkCollector odkrywa URL-e ofert na jednym listingu portalu. Dostaje kontekst
// przegladarki i sam zarzadza zakladkami: portal z paginacja moze otworzyc
// zakladke na kazda strone i ja domknac, portal z infinite scrollem trzyma jedna.
// Zwracane URL-e sa juz absolutne i uporzadkowane tak, jak maja trafic do bazy.
type LinkCollector interface {
    CollectOfferURLs(browserCtx context.Context, listingURL string) ([]string, error)
}
```

**W `internal/justjoin/transformer.go`:**

```go
// OfferParser wie, kiedy strona oferty danego portalu jest gotowa do zrzutu
// HTML-a, i jak wyciagnac z tego HTML-a pola oferty.
type OfferParser interface {
    ReadySelector() string
    ParseOffer(html, offerURL string) (ParsedOffer, error)
}
```

**Dlaczego dwa, a nie jeden `portalScraper`.** Każdy etap dostawałby metodę, której nie używa (`technical-clarifications.md` p. 3). Fetcher nie parsuje, transformer nie chodzi po listingu.

**Własność zakładki (rozstrzygnięcie M5 z audytu, 2026-09-04).** `CollectOfferURLs` dostaje **kontekst przeglądarki**, nie gotową zakładkę, i sam woła `newTab` (`browser.go:47`). Powód: Bulldogjob przechodzi trzy strony paginacji, a jedna zakładka trzymana przez wszystkie trzy dzieli jeden budżet czasu z `listingTimeout`; własna zakładka na stronę pozwala każdej dostać pełny budżet. Konsekwencja: **`listingTimeout` przestaje być jedną globalną stałą** — każda implementacja ustala własny budżet na jednostkę pracy (jedna strona albo jeden przewijany listing). To zamyka kwestię otwartą nr 4, która wcześniej nie miała jednoznacznego znaczenia.

**Dlaczego `ReadySelector()` jest w `OfferParser`, a nie osobno.** Etap pobrania HTML-a żyje w transformerze i **oba** wywołania — czekanie na gotowość strony i parsowanie — robi ten sam konsument na tym samym obiekcie. Interfejs ma dokładnie te metody, których transformer używa. Alternatywy odrzucone: trzeci interfejs (mnożenie bytów) oraz przekazywanie selektora jako pola struktury w `cmd/crawler/main.go` (rozdziela wiedzę o portalu między dwa pakiety).

**Dlaczego interfejsy są eksportowane**, mimo że `knownOfferStore` jest nieeksportowany: `cmd/crawler/main.go` musi **nazwać** te typy, żeby zbudować listę czterech pipeline'ów. Dokładnie ta sama różnica występuje już dziś między `OfferStore` (eksportowany, nazywany w `main`) a `knownOfferStore` (nieeksportowany, nigdy nienazywany poza pakietem).

**Czego NIE budujemy** (`minimal-implementation.md`): rejestru pluginów, auto-rejestracji, map selektorów sterowanych konfiguracją, generycznego „frameworka ekstraktorów", trzeciej metody „na przyszłość". Interfejs jest uzasadniony, bo potrzeba jest natychmiastowa i konkretna — cztery istniejące portale. Ograniczeniem jest jego **rozmiar**, nie istnienie.

### 9.2 Osiem implementacji, cztery pliki

Jeden plik `snake_case` na portal, zgodnie z idiomem `redis_ip_repository.go` / `in_memory_ip_repository.go`:

| Plik | Zawiera | Mechanika listingu | `slices.Reverse` |
|---|---|---|---|
| `internal/justjoin/justjoin_portal.go` | `JustjoinCollector`, `JustjoinParser` | infinite scroll (istniejąca) | **tak** |
| `internal/justjoin/nofluffjobs_portal.go` | `NofluffjobsCollector`, `NofluffjobsParser` | do ustalenia przy implementacji | **tak** |
| `internal/justjoin/bulldogjob_portal.go` | `BulldogjobCollector`, `BulldogjobParser` | paginacja, limit 3 stron / target / przebieg | **nie** |
| `internal/justjoin/solidjobs_portal.go` | `SolidjobsCollector`, `SolidjobsParser` | do ustalenia przy implementacji | **nie** |

Konstruktory: `NewJustjoinCollector()`, `NewJustjoinParser()` itd. — zwykłe funkcje zwracające wskaźnik, bez wzorca Options, zgodnie z idiomem `New<Backend><Rzecz>`.

**Kolejność wstawiania (`slices.Reverse`).** Dziś `fetcher.go:110` odwraca listę bezwarunkowo, opierając się na tym, że listing jest posortowany newest-first. Dla Bulldogjob i solid.jobs to założenie jest fałszywe (brak parametru sortowania — `approved-urls.md`, „Kwestia otwarta"). Zamiast dokładać pole-flagę do fetchera, **odwracanie staje się odpowiedzialnością kolektora**: `LinkCollector` z kontraktu zwraca URL-e „w kolejności, w jakiej mają trafić do bazy". justjoin.it i NoFluffJobs odwracają przed zwróceniem, Bulldogjob i solid.jobs zwracają kolejność listingu. To realizuje flagę per portal z F2 bez dokładania pola i bez `if`-a w `fetcher.go`.

**Bulldogjob — paginacja.** `maxListingPages = 3` stron na target na przebieg (~150 ofert). Po osiągnięciu limitu kolektor loguje `[bulldogjob] page limit (3) reached for <url>`. Limit jest **niezależny** od limitu 100 ofert na przebieg: ten ogranicza odkrywanie URL-i, tamten faktyczne scrapowanie.

**Wspólne helpery pakietu.** `scrollAndCollectLinks` (`fetcher.go:168-198`) i `collectOfferLinks` (`fetcher.go:202-212`) schodzą z roli metod `OfferFetcher` do funkcji pakietu sparametryzowanych selektorem karty oferty, wywoływanych przez te kolektory, które faktycznie mają infinite scroll. `orderedLinks` (`fetcher.go:144-160`) bez zmian.

**Normalizacja URL-a — jedno miejsce w całym systemie.**

```go
// absoluteURL rozwija href z listingu wzgledem adresu tego listingu.
func absoluteURL(listingURL, href string) (string, error)
```

- Realizacja: `url.Parse` + `(*url.URL).ResolveReference`. **Bez** obcinania query stringu i fragmentu (decyzja `url-canonicalization`, wariant A).
- Wołana **wyłącznie** w kolektorze, przed zwróceniem listy. Nigdy w transformerze, nigdy w persist.
- `href`, którego nie da się rozwinąć, jest **pomijany z logiem**, nie propagowany dalej.
- **Zakaz:** zapisywanie `location.href` po nawigacji. solid.jobs przekierowuje na adres kanoniczny; gdyby zapisany URL różnił się od tego z listingu, `unseen` przestałby trafiać i **każdy przebieg scrapowałby wszystko od nowa** — przy pauzie 2-3 min to prosta droga do bana.

**Zasada nadrzędna:** string zapisywany do `possibilities.url` musi być **bit w bit** tym samym stringiem, który trafia do `ExistingPossibilityUrls`.

### 9.3 Rozdzielenie pobrania HTML od parsowania — kontrakt obu etapów

Dziś `scrapeOffer` (`transformer.go:153-247`) robi nawigację i całą ekstrakcję w jednym `chromedp.Run`. Docelowo dwa etapy, **dla wszystkich czterech portali, łącznie z przepisanym justjoin.it** — dwa równoległe style w jednym pakiecie są niedopuszczalne (`technical-clarifications.md` p. 4).

**Etap 1 — pobranie (nieczyste, chromedp), wspólne dla wszystkich portali:**

```go
// fetchOfferHTML pobiera wyrenderowany HTML strony oferty. Zwraca HTML nawet
// wtedy, gdy oczekiwany wezel sie nie pojawil - zapisany HTML jest jedyna droga
// do pozniejszego odzyskania takiej oferty.
func fetchOfferHTML(browserCtx context.Context, offerURL, readySelector string) (string, error)
```

Kontrakt:

1. Własna zakładka przez `newTab(browserCtx, offerTimeout)` — bez zmian względem dzisiaj.
2. `chromedp.Navigate(offerURL)`.
3. **Ograniczone czasowo** czekanie na `readySelector` (`chromedp.WaitReady` w kontekście z `htmlReadyTimeout`). Przekroczenie timeoutu jest **logowane i nieblokujące** — przechodzimy do kroku 4.
4. `chromedp.Evaluate("document.documentElement.outerHTML", &html)`.
5. Zwrot HTML-a. Błąd zwracany jest tylko wtedy, gdy nie udało się pobrać **żadnego** HTML-a.

**Dlaczego krok 3 jest ograniczony czasowo i nieblokujący.** Dzisiejsze `chromedp.Text("h1", ...)` / `chromedp.Text("h2", ...)` (`transformer.go:168-169`) **blokują aż do `offerTimeout` (90 s)**, gdy węzeł nie istnieje. Przy nowej polityce „zapisz wiersz z placeholderem" (F5) byłoby to najgorsze z możliwych zachowań: 90 s zmarnowane **i** zero zapisanego HTML-a, czyli utrata dokładnie tych ofert, dla których zapis HTML-a jest najcenniejszy (nowy, niedostrojony parser). Nowy kod **nie używa `chromedp.Text` do pól obowiązkowych.**

Jednocześnie krok 3 jest niezbędny: bez żadnego czekania zrzut `outerHTML` na stronie SPA (justjoin.it, NoFluffJobs) złapałby pusty szkielet przed hydracją. Dzisiejsze `chromedp.Text("h1")` pełniło tę rolę przypadkiem — nowy kod robi to jawnie i z własnym, krótkim budżetem czasu.

**Etap 2 — parsowanie (czysta funkcja Go, zero chromedp):**

```go
type ParsedOffer struct {
    JobTitle    string
    CompanyName string
    Location    string
    IsRemote    bool
    Tech        string // wartosci rozdzielone ", " - format istniejacej kolumny tech
    Description string
    PostedAt    string // ISO YYYY-MM-DD albo "" - wejscie dla optDate
}
```

Kontrakt `ParseOffer(html, offerURL string) (ParsedOffer, error)`:

- **Determinizm.** Ten sam HTML zawsze daje ten sam wynik. Zero I/O, zero zegara, zero sieci. To jest warunek testowalności na fixture'ach.
- **Błąd** zwracany wyłącznie wtedy, gdy nie da się ustalić **pól obowiązkowych** (`JobTitle`, `CompanyName`). Brak dowolnego pola opcjonalnego to `""`/`false`, nie błąd.
- **Logowanie braku.** Gdy oczekiwane pole opcjonalne nie zostało znalezione, parser loguje to z nazwą portalu i URL-em. Dzisiejsze ciche `""` (komentarz `transformer.go:150-152`) było akceptowalne przy jednym portalu; przy czterech zamienia się w niewykrywalną degradację (`error-handling.md` → jawne sygnalizowanie zamiast cichego pomijania).
- **Normalizacja daty w Go.** Każdy parser rozpoznaje format swojego portalu i zwraca ISO `YYYY-MM-DD`. Składanie stringów daty w JS-ie (`transformer.go:218-224`) znika. `optDate` (`transformer.go:251`) i jego test zostają bez zmian.
- **Anty-wzorce, których nie wolno powielić:** `strings.Contains(body, "Remote")` (`transformer.go:242`), lokalizacja jako „linia po tytule w `innerText`" (`transformer.go:176-186`), ciche `""` przy zepsutym selektorze.

**Polityka nieudanego parsowania (F5).** Gdy `ParseOffer` zwróci błąd, oferta **i tak** trafia do bazy:

```go
// internal/offer/placeholder.go
package offer

// Unparsed oznacza wiersz zapisany mimo nieudanego parsowania: surowy HTML jest
// w possibility_html, a pola obowiazkowe dostaja te wartosc.
const Unparsed = "(unparsed)"
```

`JobTitle = offer.Unparsed`, `CompanyName = offer.Unparsed`, reszta pól pusta, HTML zapisany normalnie.

**Dlaczego osobny pakiet `internal/offer`, a nie stała w `internal/justjoin`.** Stałą muszą znać crawler **i** `internal/handlers` (do ustawienia `possibilityRow.Unparsed`). Gdyby leżała w `internal/justjoin`, `cmd/web` zaciągnąłby tranzytywnie `chromedp` — kilkadziesiąt MB zależności przeglądarkowych w binarce serwera HTTP. Pakiet z jedną stałą jest tańszy niż ta zależność. Wariant z literałem powtórzonym w `.gohtml` odrzucony w `technical-clarifications.md` p. 5 (rozjedzie się przy pierwszej zmianie brzmienia).

### 9.4 Przepływ danych i typy

**Nowe i zmienione typy w pakiecie `crawler`:**

```go
// ScrapeTarget to jeden wiersz scrape_targets, z kategoria rozwiazana juz przy
// starcie: CategorySlug pochodzi z bazy, CategoryName jest tym, co trafi do
// possibilities.category.
type ScrapeTarget struct {
    SourceID     int64
    SourceSlug   string
    CategorySlug string
    CategoryName string
    URL          string
}

type FetchedOffers struct {
    SourceID     int64
    SourceSlug   string
    CategorySlug string  // do logow
    CategoryName string  // do possibilities.category
    Pass         uint64  // numer przebiegu, do limitu 100 ofert
    Urls         []string
}

// ScrapedOffer laczy wiersz oferty z jej surowym HTML-em, bo persist zapisuje
// oba w jednym kroku.
type ScrapedOffer struct {
    Params database.CreatePossibilityParams
    HTML   string
}
```

`OfferTransformer.Offers` zmienia typ z `chan database.CreatePossibilityParams` na `chan ScrapedOffer`; `OfferPersist.Persist` przyjmuje `<-chan ScrapedOffer`.

**Przepływ (jeden portal):**

```
[start]
  db.ListEnabledScrapeTargets(ctx)
    -> grupowanie po SourceSlug
    -> category.BySlug(t.category) -> CategoryName          [nieznany slug = log + pominiecie targetu]
    -> []ScrapeTarget

[fetcher portalu X]                              Chromium #X, jedna instancja na przebieg
  Pass++
  dla kazdego targetu (sekwencyjnie):
    collector.CollectOfferURLs(tabCtx, target.URL)
      -> absoluteURL(...) dla kazdego href        * jedyny punkt normalizacji URL-a
      -> kolejnosc wg portalu (reverse albo nie)
    -> FetchedOffers{SourceID, SourceSlug, CategorySlug, CategoryName, Pass, Urls}
    -> kanal FetchedUrls (bufor 100)             [select z ctx.Done(), bez blokady shutdownu]
  pause(listingInterval + rand.N(maxListingJitter))   = 4-5 h

[transformer portalu X]                          Chromium #X', jedna instancja na batch
  reset licznika przy zmianie batch.Pass
  jesli scrapedInPass >= maxOffersPerPass: pomin batch z jednorazowym logiem
  unseen(batch) -> ExistingPossibilityUrls       * NIETKNIETE
  dla kazdego nieznanego URL-a:
    fetchOfferHTML(browserCtx, url, parser.ReadySelector()) -> html
    parser.ParseOffer(html, url) -> ParsedOffer  [blad -> placeholder, HTML zostaje]
    -> CreatePossibilityParams{Category: batch.CategoryName, SourceID: batch.SourceID, ...}
    -> kanal Offers (bez bufora) jako ScrapedOffer{Params, HTML}
    -> pause(minOfferPause + rand.N(maxOfferPause-minOfferPause))   = 2-3 min

[persist portalu X]
  ctx z timeoutem 60 s
  CreatePossibility(...) -> saved.ID             * ON CONFLICT (url) NIETKNIETE
  CreatePossibilityHtml(saved.ID, html)          [blad -> log, oferta zostaje bez HTML-a]
```

**Klucz kategorii — pułapka, która zeruje UI bez żadnego błędu.** `possibilities.category` przechowuje **nazwę wyświetlaną** (`Go`, `Software Architect`, `C/C++`), bo crawler zapisuje `cat.Name` (`fetcher.go:73`), a oba handlery odpytują po `cat.Name` (`scraped_handler.go:55`, `stats_handler.go:53`). `scrape_targets` kluczuje **slugami**. Gdyby crawler zapisał slug, obie listy pokazałyby „No scraped offers yet." i nie pojawiłby się ani jeden błąd w logach ani w monitoringu.

Ochrona jest dwuwarstwowa:
1. **Przy starcie** — każdy `scrape_targets.category` przechodzi przez `category.BySlug`; nieznany slug oznacza pominięcie targetu z logiem ostrzegawczym (`error-handling.md` → „Graceful Degradation"), nie panic.
2. **Przy zapisie** — `CreatePossibilityParams.Category` jest zasilane wyłącznie z `CategoryName`. Slug nie ma drogi do tej kolumny: `FetchedOffers` niesie oba pola, ale `CategorySlug` służy tylko logom.

**Zmiany w `internal/category/category.go`:** znika pole `ScrapeURL` i stała `scrapeParams`. `All`, `Slug`, `Name` i `BySlug` **zostają w Go bez zmian** (decyzja `category-source-of-truth`). Ripple jest zerowy — `templates/parse.go:13`, `handlers/helper.go:79`, `stats_handler.go:35`, `scraped_handler.go:37` używają wyłącznie `Slug` i `Name`.

### 9.5 Wiązanie pipeline'ów i graceful shutdown

`cmd/crawler/main.go` przechodzi z jednego kompletu goroutine'ów na **N kompletów, po jednym na portal**, gdzie N wynika z danych w bazie.

```go
targets, err := db.ListEnabledScrapeTargets(ctx)
// grupowanie po source_slug, walidacja kategorii
for _, portal := range []struct {
    slug      string
    collector crawler.LinkCollector
    parser    crawler.OfferParser
}{
    {"justjoin",    crawler.NewJustjoinCollector(),    crawler.NewJustjoinParser()},
    {"nofluffjobs", crawler.NewNofluffjobsCollector(), crawler.NewNofluffjobsParser()},
    {"bulldogjob",  crawler.NewBulldogjobCollector(),  crawler.NewBulldogjobParser()},
    {"solidjobs",   crawler.NewSolidjobsCollector(),   crawler.NewSolidjobsParser()},
} {
    // pominiecie portalu bez wlaczonych targetow, z logiem
    cr := crawler.NewOfferFetcher(portal.slug, targetsBySlug[portal.slug], portal.collector)
    ot := crawler.NewOfferTransformer(db, portal.slug, portal.parser)
    op := crawler.NewOfferPersist(db, portal.slug)
    wg.Go(func() { cr.Fetch(ctx) })
    wg.Go(func() { ot.Transform(ctx, cr.FetchedUrls) })
    wg.Go(func() { op.Persist(ot.Offers) })
}
```

Jawna, literalna lista czterech portali w `main()` — dokładnie idiom wiązania z `cmd/web/main.go:38-58`. Bez rejestru, bez fabryki, bez refleksji.

**Graceful shutdown bez podwójnego zamknięcia kanału.** Kluczowa własność: **każda gałąź ma własne kanały `FetchedUrls` i `Offers`, więc każdy kanał ma dokładnie jednego właściciela i jednego zamykacza.** Semantyka zamykania nie zmienia się ani o jotę względem dzisiejszej:

```
SIGINT/SIGTERM -> cancel(ctx)
  -> pause() zwraca false w kazdym fetcherze
  -> kazdy fetcher zamyka SWOJ FetchedUrls
  -> kazdy transformer dopina swoj batch i zamyka SWOJ Offers
  -> kazdy persist dopina i konczy petle
  -> wg.Wait() (3N goroutine'ow)
```

Wariant „N fetcherów pisze do jednego wspólnego kanału" jest **odrzucony**: dałby `panic: close of closed channel` i wymagałby dodatkowego `WaitGroup` z osobnym goroutine-zamykaczem. Wariant per-portal realizuje też wprost decyzję „osobny throttling na portal" — throttling żyje w `transformBatch`, więc osobny transformer daje go za darmo.

**Zachowania brzegowe przy starcie:**

| Sytuacja | Reakcja |
|---|---|
| `scrape_targets` puste / brak włączonych | log **ostrzegawczy** `no enabled scrape targets, crawler has nothing to do` i normalne działanie (bez pipeline'ów). Bez tego crawler startuje i cicho nic nie robi |
| Portal z bazy bez implementacji w `main()` | log + pominięcie, bez panic |
| Portal z implementacją, ale bez włączonych targetów | log + pominięcie pipeline'u |
| `scrape_targets.category` spoza `category.All` | log + pominięcie targetu |
| `ListEnabledScrapeTargets` zwraca błąd | log + panic (jak dzisiejsze zachowanie przy braku bazy — bez konfiguracji crawler nie ma co robić) |

**Zasoby.** Szczyt to 8 jednoczesnych instancji Chromium (4 fetchery × 4 transformery w najgorszym momencie) wobec 2 dzisiaj, czyli ~1,2-3,2 GB RAM. Stąd `mem_limit: 4g` i `chromedp.Flag("disable-dev-shm-usage", true)` w `newBrowser` — domyślne `/dev/shm` = 64 MB przy ośmiu instancjach niemal gwarantuje awarie kart objawiające się jako „Target closed".

### 9.6 Throttling i limity

Wszystkie cztery stałe throttlingu w **jednym bloku `const`**, w nowym pliku `internal/justjoin/throttling.go`. Bez zmiennych środowiskowych (`scope-clarifications.md` → `throttling-config`).

```go
// internal/justjoin/throttling.go

// Throttling. Kazde czekanie jest losowane na nowo, zeby crawl nie ukladal sie
// w staly rytm, ktory portal moglby rozpoznac.
const (
    listingInterval  = 4 * time.Hour    // bylo: frequency = 2h
    maxListingJitter = 1 * time.Hour    // bylo: maxFrequencyJitter = 1h  -> lacznie 4-5 h
    minOfferPause    = 2 * time.Minute  // bylo: 5s
    maxOfferPause    = 3 * time.Minute  // bylo: 12s
)

// Ograniczniki zakresu jednego przebiegu. maxOffersPerPass liczy oferty
// faktycznie zescrapowane, maxListingPages - glebokosc odkrywania URL-i.
const (
    maxOffersPerPass = 100
    maxListingPages  = 3
)
```

Zmiany nazw `frequency` → `listingInterval` i `maxFrequencyJitter` → `maxListingJitter` są celowe: nazwa ma mówić, czego dotyczy odstęp (`coding-style.md` → „Descriptive Names"). Stałe znikają z `fetcher.go:16-17` i `transformer.go:20-21`.

**Mechanizm losowania bez zmian:** `listingInterval + rand.N(maxListingJitter)` daje przedział `[4h, 5h)`, `minOfferPause + rand.N(maxOfferPause-minOfferPause)` daje `[2min, 3min)`. Anulowalne czekanie `pause(ctx, d)` (`transformer.go:135`) — bez zmian.

**Limit 100 ofert na przebieg per portal.** Uzasadnienie (`technical-clarifications.md` p. 1): przy ~2,5 min na ofertę 100 ofert to ~4h10m, czyli mniej więcej okno między przebiegami listingu — przebieg kończy się, zanim ruszy następny, więc kolejka nie narasta.

Limit liczy oferty **faktycznie zescrapowane**, a nie odkryte URL-e, więc nie może być egzekwowany w fetcherze (tam nie wiadomo jeszcze, które URL-e są nowe — `unseen` działa dopiero w transformerze). Realizacja: fetcher inkrementuje `Pass` przy każdym przebiegu i wpisuje go w każdy `FetchedOffers`; transformer trzyma licznik `scrapedInPass`, zeruje go przy zmianie numeru przebiegu i po przekroczeniu limitu pomija pozostałe batche tego przebiegu z jednorazowym logiem. Pominięte targety zostaną odkryte ponownie w kolejnym przebiegu.

**Rotacja kolejności targetów (rozstrzygnięcie M7 z audytu, 2026-09-04).** Przy stałej kolejności (`ORDER BY s.slug, st.category`) limit 100 zjadałyby targety z początku listy. Konkretnie dla Bulldogjoba: `ai-ml` i `architect` zwracają po ~50 ofert każdy (`approved-urls.md`), więc razem wyczerpują cały limit portalu, a `go`, `php` i oba warianty `c` nie dostają **nic** przez wiele dni backfillu.

Fetcher rotuje więc kolejność targetów w obrębie portalu między przebiegami — przebieg `N` zaczyna od targetu o indeksie `N mod len(targets)`. Rozwiązanie jest bezstanowe (numer przebiegu i tak jest już potrzebny do limitu), więc nie wymaga niczego w bazie. Po nadrobieniu backfillu limit przestaje się aktywować i rotacja staje się nieistotna — ale w pierwszych dniach decyduje o tym, czy dane napływają równomiernie.

**Pozostałe timeouty:**

| Stała | Dziś | Docelowo | Powód |
|---|---|---|---|
| timeout zapisu w `persist.go:32` | `3 * time.Second` | **`60 * time.Second`** | dwa `INSERT`-y bez transakcji, drugi z setkami KB TEXT-a, cztery pipeline'y konkurujące o pulę `pgxpool` bez jawnej konfiguracji rozmiaru. Duży zapas sprawia, że timeout praktycznie przestaje być przyczyną utraty danych; przy pauzie 2-3 min minuta blokady nie jest wąskim gardłem |
| `offerTimeout` | `90 * time.Second` | bez zmian | budżet na całą stronę oferty |
| `htmlReadyTimeout` | — | **nowa, ok. 15-20 s** | ograniczone czekanie na `ReadySelector`; przekroczenie nie przerywa zrzutu HTML-a |
| `listingTimeout` | `3 * time.Minute` | **przenoszony do implementacji `LinkCollector`** | po rozstrzygnięciu M5 kolektor sam otwiera zakładki, więc budżet czasu należy do jednostki pracy (jedna strona paginacji albo jeden przewijany listing), a nie do całego targetu. Globalna stała znika |
| `knownOffersTimeout` | `5 * time.Second` | **`30 * time.Second`** | ta sama przesłanka co przy zapisie: cztery pipeline'y konkurują o pulę `pgxpool` bez jawnej konfiguracji rozmiaru. Przekroczenie jest tu **kosztowniejsze** niż przy zapisie — `transformer.go:110-112` zwraca wtedy cały batch jako nieznany, więc crawler scrapuje po raz drugi oferty, które już ma, marnując godziny żądań do portalu. Rozstrzygnięcie H5 z audytu, 2026-09-04 |

**Bufor `visibleOffers = 100`** — bez zmian. Przy zapełnieniu bufora fetcher blokuje się na wysyłce, ale `select` z `ctx.Done()` (`fetcher.go:78-82`) sprawia, że shutdown nadal działa. Deadlocku nie ma; jest ciche zatrzymanie odświeżania listingów. Do udokumentowania, nie do naprawiania.

### 9.7 Logowanie

**Wymóg, nie decyzja.** Dziś prefiks loga to sama kategoria (`transformer.go:74,110,126`, `fetcher.go:72,112`). Przy czterech równoległych pipeline'ach `[Go 3/50] Scraping: ...` nie mówi, który portal jest crawlowany, i diagnoza czegokolwiek na produkcji staje się niemożliwa.

Każda linia loga crawlera dostaje slug portalu: `[justjoin/go 3/50] Scraping: <url>`. Dotyczy to również logów kolektorów (limit stron), parserów (brakujące pole), fetchera (start przebiegu, czekanie) i persist (zapisano / błąd). Bez poziomów i bez logowania strukturalnego — stdlib `log`, zgodnie z idiomem repo.

---

## 10. Projekt wizualny

Mockupy w `analysis/design-context/` są **wiążącym wejściem** dla części UI. `implementation-planner` dołącza sekcję `Visual References` do grupy zadań UI, wskazując `analysis/design-context/ascii/ui-mockups.md`.

**Fidelity:** ASCII, orientacyjne. Wiążące są: **liczba i kolejność kolumn, klasy Bootstrapa i wartość `colspan`**; proporcje szerokości nie są wiążące (wynikają z treści i z `table-responsive`).

### 10.1 Ekrany

| ID | Plik | Zmiana |
|---|---|---|
| `screen:scraped-list` | `internal/templates/scraped/list.gohtml` | 4 → **5 kolumn**, `Source` na pozycji **drugiej**, między `Title` a `Published` |
| `screen:scraped-list-before` | — | punkt odniesienia dla diffa, bez zmian w kodzie |
| `screen:scraped-list-empty` | `internal/templates/scraped/list.gohtml:29` | `colspan="4"` → **`colspan="5"`** |
| `screen:category-nav` | `internal/templates/partials/categorynav.gohtml` | **bez zmian** (operuje tylko na `.Category.Name`/`.Category.Slug` i `.Page`) |

### 10.2 Komponenty

| ID | Zmiana |
|---|---|
| `component:source-column-header` | nowy `<th>Source</th>` bezpośrednio po `<th>Title</th>` (`scraped/list.gohtml`, po linii 13) |
| `component:source-cell` | nowy `<td class="text-nowrap">` po linii 22, z `<span class="badge text-bg-light border">{{ .Source }}</span>`. `text-nowrap`, żeby `nofluffjobs.com` się nie łamał |
| `component:possibility-row` | `possibilityRow` (`internal/handlers/scraped_handler.go:28-33`) dostaje `Source string` **i** `Unparsed bool` |
| `component:row-no-published-date` | **zmiana niewymagana** — `formatDate` (`helper.go:36-41`) już zwraca `-` dla `pgtype.Date{Valid:false}` |
| `component:row-duplicate-across-sources` | **zmiana niewymagana** — wynika z pozycji kolumny; to ten wariant przesądza o umieszczeniu `Source` obok `Title` |
| `component:row-unparsed-placeholder` | komórka tytułu renderuje `text-muted fst-italic` zamiast `fw-medium`, gdy `.Unparsed`. Wariant **B** z mockupu (`technical-clarifications.md` p. 5) |
| `component:source-null-fallback` | `source_id IS NULL` renderuje gołe `-`, **bez badge'a** |
| `component:empty-state-row` | `colspan="5"` |

### 10.3 Bez zmian (potwierdzone)

`component:categorynav`, `component:header-nav`, `component:page-head`, `component:page-footer`, `screen:stats` — żaden nie odwołuje się do portali ani do `possibilities.source_id`. Filtrowanie po źródle i statystyki per źródło są poza zakresem.

### 10.4 Warstwa Go dla UI

`internal/handlers/scraped_handler.go`:

- `PossibilityStore.ListPossibilitiesByCategory` zwraca `[]database.ListPossibilitiesByCategoryRow`.
- `possibilityRow` zyskuje `Source string` (z `formatText(p.SourceName)` — daje `-` dla `NULL`, spójnie z resztą aplikacji) i `Unparsed bool` (`p.JobTitle == offer.Unparsed`).
- **Nazwa portalu pochodzi z `LEFT JOIN sources` w zapytaniu**, nie z mapy w handlerze (`technical-clarifications.md` p. 6). Mapa wprowadzałaby cache, którego nie da się odświeżyć bez restartu aplikacji, a źródła zmienia się przez `psql`.

Szablon rozróżnia badge od myślnika idiomem `{{ if eq }}` z `offers/list.gohtml:36-45`:

```gohtml
<td class="text-nowrap">
    {{ if eq .Source "-" }}-{{ else }}<span class="badge text-bg-light border">{{ .Source }}</span>{{ end }}
</td>
```

**Nowy CSS: zero. Nowe partiale: zero. Nowy JS: zero.** Wszystko to istniejące utilities Bootstrapa.

**Skala listy.** Lista rośnie z ~150 do ~600 wierszy (4 portale + duplikaty między portalami). Paginacja jest poza zakresem: `table-responsive` i `table-hover` działają przy dowolnej liczbie wierszy, render jest po stronie serwera, użytkownik jest jeden. Kolumna `Source` niczego tu nie pogarsza — bez niej lista 600 wierszy z czterech portali byłaby nieczytelna.

---

## 11. Analiza ponownego użycia

### 11.1 Istniejący kod do wykorzystania

| Element | Lokalizacja | Zastosowanie w tym zadaniu |
|---|---|---|
| Idiom wąskiego interfejsu u konsumenta | `transformer.go:28`, `persist.go:11`, `router/routing.go:50-58` | wzorzec dla `LinkCollector` i `OfferParser` |
| Idiom „jeden plik na implementację + `New<X>` + wybór w `main()`" | `internal/repository/*_ip_repository.go`, `cmd/web/main.go:38-58` | struktura czterech plików portali i wiązania w `cmd/crawler/main.go` |
| `pause(ctx, d)` — anulowalne czekanie | `transformer.go:135-145` | bez zmian, nowe wartości throttlingu |
| `rand.N` — losowanie odstępu | `fetcher.go:50`, `transformer.go:86` | bez zmian, mechanizm jest już dokładnie taki, jak wymagany |
| `newBrowser` / `newTab` — cykl życia chromedp | `browser.go:20,47` | bez zmian; `newTab` woła teraz również implementacja `LinkCollector` (M5) |
| `orderedLinks` | `fetcher.go:144-160` | bez zmian, wspólne dla kolektorów |
| `scrollAndCollectLinks` / `collectOfferLinks` | `fetcher.go:168-212` | przeniesione do helperów pakietu, sparametryzowane selektorem |
| `optDate` / `optText` | `transformer.go:251-266` | bez zmian; parsery zwracają ISO i `""`, więc oba helpery działają dalej |
| `unseen` + `ExistingPossibilityUrls` | `transformer.go:100-130` | **bez zmian w zachowaniu** — objęte zakazem regresji |
| `WithTx` w `generated/db.go` | — | **nieużywane i takie zostaje** — transakcja odrzucona w Fazie 2 |
| Badge `text-bg-light border` | `offers/list.gohtml:40,44` | render nazwy portalu |
| `{{ if eq … }}` w `<td>` | `offers/list.gohtml:36-45` | rozróżnienie badge / `-` oraz `fw-medium` / `text-muted fst-italic` |
| `text-nowrap`, `text-muted` | `scraped/list.gohtml:23,29` | komórka `Source`, wiersz placeholdera |
| `formatDate` / `formatText` → `-` | `helper.go:36,64` | pusta data, pusty stack, brak źródła |
| `card shadow-sm` + `table-responsive` + `table table-striped table-hover align-middle mb-0` | `scraped/list.gohtml:8-10` | kontener tabeli, obsługa 5 kolumn i ~600 wierszy |
| `category.All` / `BySlug` | `internal/category/category.go` | zostają; znika wyłącznie `ScrapeURL` |
| Idiom testów tabelarycznych ze stubami | `transformer_test.go` | testy parserów i `absoluteURL` |
| Guard `current_schema()` w idempotentnym DDL | `deploy.sh:62` | wzorzec dla asercji i migracji |
| `pg_dump` przed deployem | `deploy.sh:24-29` | jedyna ścieżka rollbacku |
| `restart: always`, `init: true` | `docker-compose.yaml:48,51` | bez zmian |

### 11.2 Nowy kod i jego uzasadnienie

| Nowy element | Dlaczego nie da się użyć istniejącego |
|---|---|
| `LinkCollector` + 4 implementacje | Bulldogjob ma paginację zamiast infinite scrolla, więc różnica leży w **logice sterującej**, nie w danych. Sparametryzowany selektor `.offer-card` nie zadziała; wspólny kod rozjechałby się w `if`-y |
| `OfferParser` + 4 implementacje | każdy portal ma inny DOM, inny format daty i inne miejsce na stack; jedna funkcja z gałęziami byłaby nietestowalna i nieczytelna |
| `fetchOfferHTML` | dzisiejszy `scrapeOffer` nie ma seamu pobranie/parsowanie; bez rozcięcia zapis surowego HTML-a i testy parserów są niemożliwe |
| `ParsedOffer` | `database.CreatePossibilityParams` jest strukturą insertu z `sqlc`; parser nie powinien znać `pgtype` ani kolumn bazy |
| `ScrapedOffer` | kanał musi nieść HTML obok parametrów insertu, bo persist zapisuje oba |
| `absoluteURL` | dziś `node.Attribute("href")` idzie prosto do bazy; nie ma żadnej normalizacji do ponownego użycia |
| tabele `sources`, `scrape_targets`, `possibility_html` | pojęcie źródła nie istnieje w żadnej warstwie (`grep source` w całym repo: 3 trafienia, wszystkie nieistotne) |
| `ListEnabledScrapeTargets`, `CreatePossibilityHtml` | brak jakiegokolwiek zapytania o źródła i o HTML |
| `internal/offer` (jedna stała) | stałą muszą znać crawler i handlery; wspólne miejsce w `internal/justjoin` wciągnęłoby `chromedp` do binarki web |
| `internal/database/migrations/001_...sql`, `seed.sql` | dziś DDL to nadpisywany heredoc, a seedu nie ma w ogóle |
| `internal/justjoin/throttling.go` | cztery stałe są dziś rozrzucone po dwóch plikach |

### 11.3 Świadomie **nie**budowane

Rejestr pluginów, auto-rejestracja portali, mapa selektorów sterowana konfiguracją, generyczny „framework ekstraktorów", trzeci interfejs na readiness, `updated_at` w tabelach, których nikt nie edytuje, osobny indeks na `scrape_targets.source_id` (pokryty przez `UNIQUE`), CRUD źródeł, filtr po źródle, polityka retencji HTML-a, zmienne `CRAWLER_*`, tabela `schema_migrations`, użycie `WithTx`.

---

## 12. Zgodność ze standardami

| Standard | Jak realizowany | Świadome odstępstwa |
|---|---|---|
| `backend/migrations.md` | „Descriptive Names" — `001_multi_portal_sources.sql`; „Separate Schema and Data" — DDL osobno od `seed.sql` i od obu kroków `UPDATE`; „Small and Focused" — jeden plik = jedna logiczna zmiana; „Version Control" — plik append-only | **„Reversible"** — brak skryptu rollbacku; odwrotem jest `pg_dump` z `deploy.sh:24-29`. **Brak ewidencji zastosowanych migracji** — świadomie, przy pełnej idempotencji |
| `backend/models.md` | patrz tabela w [5.2](#52-uzasadnienia-wobec-standardsbackendmodelsmd) | `possibilities.source_id` **nullable** wbrew „Database Constraints" — nie da się pogodzić z istniejącymi danymi bez backfillu; dociśnięcie do `NOT NULL` to osobne, późniejsze zadanie. `scrape_targets.category` bez FK — kategorie zostają w Go |
| `backend/queries.md` | „Parameterized Queries" — całość przez `sqlc`; „Avoid N+1" — jeden `ListEnabledScrapeTargets`, jeden `ExistingPossibilityUrls` na batch, `LEFT JOIN` zamiast N lookupów; „Select Only Needed Columns" — `ListPossibilitiesByCategory` porzuca `SELECT *`; „Index Strategic Columns" — `possibilities_source_id_idx`, istniejący indeks kategorii nietknięty; „Query Timeouts" — każde zapytanie w `context.WithTimeout` | **„Transactions"** — zapis oferty i HTML-a idzie dwoma osobnymi `INSERT`-ami. Transakcja odrzucona w Fazie 2; skutek w [15](#15-ryzyka-i-świadomie-przyjęte-długi-techniczne) |
| `global/minimal-implementation.md` | interfejsy mają po 1-2 metody, każda z natychmiastowym wołającym; brak rejestrów i fabryk; brak `updated_at` bez pisarza; brak redundantnego indeksu; `WithTx` zostaje nieużywane zamiast być użyte „bo jest" | Interfejsy per portal **są** uzasadnione: potrzeba jest natychmiastowa i konkretna (cztery istniejące portale), nie hipotetyczna. Ograniczeniem jest ich rozmiar, nie istnienie |
| `global/error-handling.md` | „Graceful Degradation" — nieznany slug portalu/kategorii = log + pominięcie, nie panic; nieudane parsowanie = wiersz z placeholderem zamiast utraty danych; błąd zapisu HTML-a nie kasuje oferty. „Fail Fast" — walidacja slugów przy starcie, asercja `current_schema()` przed DDL. „Error Wrapping" — `fmt.Errorf("...: %w", err)` dla błędów z chromedp i pgx. „Resource Cleanup" — `defer closeBrowser()`, `defer cancelTab()` bez zmian | — |
| `global/coding-style.md` | pakiety lowercase; pliki `snake_case.go`; typy `<Portal><Rola>` PascalCase; konstruktory `New<X>(...) *X`; DRY — stała placeholdera w jednym miejscu, `absoluteURL` w jednym miejscu, seed w jednym pliku; brak dead code | `frequency`/`maxFrequencyJitter` przemianowane dla czytelności |
| `global/commenting.md` | komentarze „dlaczego", nie „co" — utrzymana mocna konwencja repo (wirtualizacja listingu, kolejność wstawiania, powód użycia `Evaluate`). **Zero komentarzy w stylu change-log** („dodane w migracji 001", „było 3 s") | — |
| `global/validation.md` | walidacja slugów kategorii przy starcie; `CHECK (category <> '')` i `UNIQUE` w bazie — walidacja wielowarstwowa; allowlist czterech portali literalnie w `main()` zamiast dowolnego sluga z bazy | — |
| `global/conventions.md` | „Interface-Based Dependency Injection" — wiązanie w `main()`; „Build What You Need"; „Predictable Structure" — nowe pliki w istniejących katalogach | **„Environment Variables"** — throttling zostaje `const`. Odstępstwo zatwierdzone w Fazie 2; zebranie czterech stałych w jeden blok daje 90% korzyści przy zerowym koszcie |
| `testing/test-writing.md` | „Test Behavior" — parsery testowane przez wejście/wyjście na fixture'ach, nie przez selektory; „Clear Names" — podtesty jako zdania po angielsku, idiom repo; „Mock External Dependencies" — stuby zamiast chromedp i Postgresa; „Fast Execution" — parsery to czyste funkcje; „Risk-Based" — `absoluteURL` i parsery to miejsca o najwyższym ryzyku | **Brak testu integracyjnego** dla `ON CONFLICT (url)` — odrzucony w Fazie 2, mimo że mechanizm jest objęty zakazem regresji. Patrz [15](#15-ryzyka-i-świadomie-przyjęte-długi-techniczne) |

---

## 13. Plan testów

Wyłącznie stdlib `testing`, tabele `[]struct{name; ...; want}` + `t.Run`, ręcznie pisane stuby (bez `testify`, bez frameworka mockującego). Nazwy podtestów to zdania po angielsku (`"keeps the order of the new offers"`), nie forma `shouldReturnXWhenY`. **2-8 testów na grupę zadań.**

### 13.1 Fixture'y HTML

Nowa konwencja w repo (dziś nie istnieje): `internal/justjoin/testdata/<portal>/<przypadek>.html`. Zawartość to realny, zapisany HTML oferty z danego portalu — pozyskany dokładnie tą drogą, którą zapisuje etap 1, więc fixture i produkcja widzą to samo.

```
internal/justjoin/testdata/
    justjoin/{typical,no-date,no-stack,remote,unparsable}.html
    nofluffjobs/{typical,no-date,no-stack,remote,unparsable}.html
    bulldogjob/{typical,no-date,no-stack,remote,unparsable}.html
    solidjobs/{typical,no-date,no-stack,remote,unparsable}.html
```

`unparsable.html` to fixture syntetyczny (np. strona błędu portalu albo szkielet SPA bez hydracji) — reszta to realne strony.

### 13.2 Grupy testów

| Grupa | Plik | Testy | Przypadki |
|---|---|---|---|
| Normalizacja URL-a | `fetcher_test.go` | `TestAbsoluteURL` | href relatywny od korzenia, href relatywny względem ścieżki, href już absolutny, href z query stringiem i fragmentem (**musi zostać zachowany**), href z `+` w ścieżce (`/pl/c++`), href pusty / niedający się rozwinąć (pominięcie, nie panic) |
| Parser justjoin.it | `justjoin_portal_test.go` | `TestJustjoinParseOffer` | oferta typowa (wszystkie pola), brak daty publikacji, brak stacku technologicznego, oferta zdalna, oferta nieparsowalna (błąd) |
| Parser NoFluffJobs | `nofluffjobs_portal_test.go` | `TestNofluffjobsParseOffer` | jw. |
| Parser Bulldogjob | `bulldogjob_portal_test.go` | `TestBulldogjobParseOffer` | jw. |
| Parser solid.jobs | `solidjobs_portal_test.go` | `TestSolidjobsParseOffer` | jw. |
| Daty per portal | w plikach parserów | pokryte przez fixture'y | format każdego portalu → ISO; `TestOptDate` (`transformer_test.go`) zostaje bez zmian |
| `unseen` | `transformer_test.go` | `TestUnseen` (rozszerzony) | istniejące przypadki + **scenariusz wieloportalowy**: ta sama nazwa oferty pod różnymi URL-ami musi zostać potraktowana jako różne oferty (deduplikacja semantyczna jest poza zakresem) |
| Limit ofert na przebieg | `transformer_test.go` | `TestOffersPerPassLimit` | licznik zeruje się przy zmianie `Pass`; batche ponad limit są pomijane |

**Wspólne wymagania dla każdego testu parsera:**

- `Tech` w formacie rozdzielanym `", "` — ten sam, którego używa `StatsPossibilityByCategory` przez `string_to_array(tech, ',')`.
- `PostedAt` w ISO `YYYY-MM-DD` albo `""`.
- Brak pola opcjonalnego daje wartość zerową, **nie** błąd.
- Brak pola obowiązkowego daje błąd, który wywołujący zamienia na `offer.Unparsed`.

### 13.3 Czego testy NIE pokrywają (świadomie)

- `ON CONFLICT (url) DO UPDATE` i `ExistingPossibilityUrls` przeciw realnemu Postgresowi — test integracyjny odrzucony w Fazie 2, **przy jawnym zakazie regresji tego mechanizmu**.
- Kolektory (`CollectOfferURLs`) — wymagają żywego chromedp i żywego portalu.
- Zapis `possibility_html` i timeout 60 s.
- Warstwa szablonów.

**Konsekwencja:** testy parserów są **jedynym** pokryciem tej zmiany i jednocześnie jedynym mechanizmem, który wykryje, że któryś portal przemodelował DOM.

---

## 14. Kryteria akceptacji

**Schemat i dane**

1. `internal/database/schema.sql` i `internal/database/migrations/001_multi_portal_sources.sql` opisują ten sam stan docelowy — potwierdzone porównaniem `\d sources`, `\d scrape_targets`, `\d possibility_html`, `\d possibilities` na bazie zbudowanej ze `schema.sql` i na bazie po migracji.
2. Żaden plik SQL w repozytorium nie zawiera literału `public` w kontekście kwalifikacji obiektu.
3. `SELECT count(*) FROM scrape_targets` zwraca `23`, `SELECT count(*) FROM sources` zwraca `4`.
4. Ponowne uruchomienie migracji i `seed.sql` na tej samej bazie nie zmienia liczby wierszy ani nie zwraca błędu (idempotencja).

**Zakaz regresji**

5. **Dedup po URL zachowany** (przeformułowane po R2, 2026-09-04). Weryfikowane są trzy rzeczy, a nie dosłowna niezmienność linii:
    - `possibilities.url TEXT NOT NULL UNIQUE` nietknięte — `git diff` na `internal/database/schema.sql` nie pokazuje zmiany tej linii;
    - cel konfliktu to nadal `ON CONFLICT (url)` — **nie** `ON CONFLICT (source_id, url)`;
    - dla oferty poprawnie sparsowanej powtórny zapis zmienia **wyłącznie** `posted_at`, dokładnie jak dziś. Siedem klauzul `CASE` dodanych w [5.3](#53-czego-nie-wolno-ruszyć) aktywuje się tylko wtedy, gdy wiersz w bazie ma `job_title = '(unparsed)'`.

    Zmiany oczekiwane i dozwolone: `source_id` / `$10` w liście `INSERT`, klauzule `CASE` w `DO UPDATE SET`, oraz warunek `AND job_title <> '(unparsed)'` w `ExistingPossibilityUrls`.

    **Dlaczego przeformułowane:** poprzednie brzmienie żądało niezmienionych linii `ON CONFLICT ... SET`, co jest wprost sprzeczne z zatwierdzonym H4 — kryterium byłoby niespełnialne. Pierwotne polecenie użytkownika („dedup po URL nietknięty") chroniło **zachowanie** deduplikacji, nie literalny tekst zapytania.
6. `SELECT DISTINCT category FROM possibilities` po pełnym przebiegu crawlera zwraca wyłącznie `Go`, `PHP`, `Software Architect`, `AI/ML`, `C/C++` — żadnego sluga.
7. `SELECT count(*) FROM possibilities WHERE url NOT LIKE 'http%'` zwraca `0`.
8. `/scr4p3d/{category}` i `/st4ts/{category}` odpowiadają `200` dla wszystkich pięciu kategorii, zarówno przed backfillem `source_id`, jak i po nim.

**Crawler**

9. Uruchomienie crawlera startuje cztery niezależne pipeline'y; log każdej linii zawiera slug portalu.
10. `SIGTERM` kończy proces bez `panic: close of closed channel` i bez zawieszenia — potwierdzone w trakcie aktywnego scrapowania.
11. Pusta tabela `scrape_targets` daje log ostrzegawczy, a nie ciche milczenie; slug portalu bez implementacji daje log i pominięcie, a nie panic.
12. Odstęp między dwoma kolejnymi ofertami mieści się w `[2min, 3min)`, a między dwoma przebiegami listingu w `[4h, 5h)` — potwierdzone znacznikami czasu w logu.
13. Żaden pipeline nie scrapuje więcej niż 100 ofert w jednym przebiegu.
14. Dla każdej oferty zapisanej **po migracji** istnieje wiersz w `possibility_html`:
    ```sql
    SELECT count(*) FROM possibilities p
    LEFT JOIN possibility_html h ON h.possibility_id = p.id
    WHERE h.possibility_id IS NULL
      AND p.created_at > (SELECT created_at FROM sources WHERE slug = 'justjoin');
    ```
    Oczekiwane `0`. Wartość `> 0` oznacza sieroty po nieudanym drugim `INSERT`-cie (patrz [15](#15-ryzyka-i-świadomie-przyjęte-długi-techniczne)).

    **Korekta po audycie (H3, 2026-09-04):** poprzednia wersja warunkowała sprawdzenie na `p.source_id IS NOT NULL`, co po backfillu z kroku 6 obejmowałoby **całą historyczną bazę justjoin.it** — która z definicji nigdy nie będzie miała HTML-a. Kryterium dawałoby wtedy fałszywy alarm przy każdym uruchomieniu i jedyny mechanizm wykrywania skutków odrzuconej transakcji byłby bezużyteczny. Odcięcie po `created_at` względem momentu utworzenia wiersza `sources` (czyli momentu seedu) zawęża sprawdzenie do ofert pobranych już nowym pipeline'em.
15. Oferta, której nie da się sparsować, ma wiersz w `possibilities` z `job_title = '(unparsed)'` **oraz** niepusty wiersz w `possibility_html`.

**UI**

16. `/scr4p3d/go` renderuje pięć kolumn w kolejności `Title, Source, Published, Technologies, URL`.
17. Wiersz z `source_id IS NULL` pokazuje gołe `-` w kolumnie `Source`, bez elementu `<span class="badge">`.
18. Wiersz z `job_title = '(unparsed)'` ma komórkę tytułu z klasami `text-muted fst-italic`.
19. Pusta kategoria renderuje komunikat „No scraped offers yet." w komórce z `colspan="5"`, wyśrodkowany względem całej tabeli.
20. `git diff` na `internal/templates/` nie dodaje ani jednej reguły CSS, ani jednego pliku JS, ani jednego nowego partiala.

**Jakość**

21. `go build ./...`, `go vet ./...` i `go test ./...` przechodzą.
22. Każdy z czterech parserów ma test tabelaryczny obejmujący co najmniej: ofertę typową, brak daty, brak stacku, ofertę zdalną, ofertę nieparsowalną.
23. `absoluteURL` ma test pokrywający zachowanie query stringu i fragmentu.
24. `go list -deps ./cmd/web | grep chromedp` nie daje trafień — stała placeholdera nie wciągnęła zależności przeglądarkowych do binarki web.

    **Korekta po audycie (M3, 2026-09-04):** poprzednia wersja opierała się na `grep` po źródłach, który wykrywa wyłącznie import bezpośredni. Zależność tranzytywna — `cmd/web` → `internal/handlers` → pakiet importujący `chromedp` — przeszłaby niezauważona. `go list -deps` rozwija pełne domknięcie zależności i jest jedynym wiarygodnym sprawdzeniem tego kryterium.

---

## 15. Ryzyka i świadomie przyjęte długi techniczne

### 15.1 Ryzyka krytyczne z aktywną ochroną

| Ryzyko | Mechanizm cichej awarii | Ochrona w tej specyfikacji |
|---|---|---|
| **Zapisanie sluga zamiast `Name`** do `possibilities.category` | oba widoki pokazują „No scraped offers yet." **bez jednego błędu w logach** | `CategoryName` rozwiązywany przy starcie i niesiony przez `FetchedOffers`; slug nie ma drogi do kolumny; kryterium akceptacji 6 |
| **Relatywne vs absolutne URL-e** | dwa portale mogą wyemitować identyczną ścieżkę → kolizja `UNIQUE (url)` **między portalami**; `ON CONFLICT DO UPDATE` zwróci sukces, oferta znika bez śladu | `absoluteURL` w jednym miejscu, w kolektorze; kryterium akceptacji 7 |
| **Zmiana kanonizacji po fakcie** | jeśli istniejące wiersze są relatywne, wymuszenie absolutnych tworzy komplet duplikatów całej historii | obowiązkowa weryfikacja produkcji **przed** DDL ([sekcja 4](#4-kroki-wymagane-przed-napisaniem-ddl)) + warunkowy jednorazowy `UPDATE` |
| **Zapis `location.href` po przekierowaniu** (solid.jobs) | `unseen` przestaje trafiać, **każdy przebieg scrapuje wszystko od nowa** → przy 2-3 min/oferta prosta droga do bana | jawny zakaz w [9.2](#92-osiem-implementacji-cztery-pliki); zapisywany jest URL z listingu |
| **DDL trafia do `public` zamiast `wok`** | tabele powstają tam, gdzie aplikacja ich nie widzi | wyłącznie niekwalifikowane nazwy + `current_schema()`; asercja w `deploy.sh` przed DDL |
| **`colspan="4"` przeoczone** | pusty stan przestaje być wyśrodkowany | jawnie w `screen:scraped-list-empty`; kryterium akceptacji 19 |
| **`chromedp.Text` blokujące 90 s** przy braku węzła | zmarnowane 90 s **i** utrata HTML-a dokładnie tam, gdzie jest najcenniejszy | etap 1 nie używa `Text` do pól obowiązkowych; czekanie ograniczone `htmlReadyTimeout` i nieblokujące |

### 15.2 Świadomie przyjęte długi techniczne

| Dług | Skutek | Decyzja |
|---|---|---|
| **Brak transakcji przy zapisie oferty i HTML-a** | dwa osobne `INSERT`-y: jeśli drugi padnie (timeout, restart kontenera między zapisami), zostaje **wiersz oferty bez zapisanego HTML-a** — dokładnie ta sytuacja, przed którą miał chronić punkt (c) zadania. **Brak automatycznego wykrywania takich sierot**; jedyną drogą jest ręczne zapytanie z kryterium akceptacji 14 | odrzucone w Fazie 2 (`persist-tx-wiring`) |
| **Zero testów dedupu po URL** | `ON CONFLICT (url) DO UPDATE` jest objęty jawnym zakazem regresji i nie ma **żadnego** pokrycia. Żaden test nie wykryje, jeśli zmiana w warstwie zapisu po cichu zepsuje deduplikację | test integracyjny odrzucony w Fazie 2 |
| **`possibility_html` write-only, bez retencji** | `deploy.sh:24-29` robi pełny `pg_dump` przy **każdym** deployu. Tysiące ofert × setki KB → każdy kolejny deploy wolniejszy, backup rośnie nieograniczenie | zaakceptowane; kasowanie ręczne, gdy zaboli |
| **Brak CRUD-u źródeł** | wyłączenie pary portal × kategoria wymaga `sudo docker compose exec postgres psql`. Wciąż lepiej niż rekompilacja, ale nie „bez dotykania serwera" | zaakceptowane (`sources-crud-orphan`) |
| **Rozmycie statystyk `tech`** | cztery portale wniosą „JS" / „JavaScript" / „Javascript" do jednej agregacji `StatsPossibilityByCategory`; liczby przestaną się sumować sensownie | zaakceptowane; surowy HTML pozwala odzyskać to później |
| **Brak rollbacku migracji** | odwrotem jest wyłącznie `pg_dump` sprzed deployu | `migrations.md` → odstępstwo od „Reversible" |
| **`make db-reset` niebezpieczny na VPS** | `DROP SCHEMA public CASCADE` nic nie usuwa w schemacie `wok`, a `schema.sql` wywala się na istniejących tabelach; po dodaniu trzech tabel gorzej. Cel jest już dziś zepsuty, to zadanie tylko podnosi stawkę | naprawa poza zakresem; **nie używać na VPS** |
| **Schemat `wok` poza kontrolą wersji** | odtworzenie produkcji z samego repozytorium jest niemożliwe | poza zakresem |
| **Ta sama oferta w dwóch targetach jednego portalu** | `ON CONFLICT` zostawia `category` z pierwszego wpisu, więc oferta pojawi się tylko w jednej kategorii (NoFluffJobs `/pl/go` + `/pl/golang`; Bulldogjob `role,ai` + `skills,Go`). Zachowanie istnieje już dziś, po prostu się nie ujawniało | zgodne z „dedup po URL nietknięty"; nie naprawiamy |
| **Kolejność listingu dla Bulldogjob i solid.jobs** | brak parametru sortowania → kolejność wstawiania jest arbitralna, a `created_at DESC` przestaje oznaczać „najnowsze na górze" dla połowy portali. Przy limicie 3 stron paginacji może to **systematycznie pomijać świeże oferty** | zaakceptowane; kolumna `Source` czyni tę niemonotoniczność zrozumiałą dla użytkownika |
| **Bufor `FetchedUrls` = 100 batchy** | przy zaległościach transformera fetcher zablokuje się na wysyłce po ~3 dniach. Deadlocku nie ma (`select` z `ctx.Done()`), jest ciche zatrzymanie odświeżania listingów | udokumentowane, nie naprawiane |
| **Długi pierwszy backfill** | ~doba przy optymistycznych założeniach, kilka dni przy pesymistycznych; przy limicie 100 ofert/przebieg rozłoży się na kilka dni | stan przejściowy, świadomie zaakceptowany |
| **Brak User-Agenta i obsługi anty-bot** | domyślny UA headless Chromium jest powszechnie wykrywany; brak proxy, CAPTCHA, ciasteczek. Nowe portale mogą być mniej tolerancyjne niż justjoin.it | ryzyko zewnętrzne, poza zakresem; throttling 2-3 min jest jedyną ochroną |
| **Czas życia jednej instancji Chromium** | z ~5-10 min rośnie do ~4 h (100 ofert × 2,5 min); wyciek pamięci Chromium staje się realny | `mem_limit: 4g` łagodzi skutek; recykling przeglądarki co N ofert — [16](#16-kwestie-otwarte) poz. 5 |

---

## 16. Kwestie otwarte

Rzeczy, których **nie dało się rozstrzygnąć** na podstawie zatwierdzonych artefaktów. Żadna nie została zgadnięta.

**Stan na 2026-09-04, po audycie specyfikacji:** rozstrzygnięte są pozycje **1, 4, 7, 8 i 9**. Otwarte pozostają **2, 3, 5 i 6**:

- **2 i 3** z natury wymagają dostępu do DOM-u portali — zamkną się przy implementacji odpowiednich `LinkCollector`/`OfferParser`.
- **5** (recykling Chromium) — decyzja strojeniowa, do podjęcia po pierwszym przebiegu.
- **6** (rozmiar puli `pgxpool`) — objaw złagodzony przez podniesienie `knownOffersTimeout` do 30 s i timeoutu zapisu do 60 s (H5), ale przyczyna nietknięta. `internal/config/loader.go` pozostaje poza inwentarzem plików tej specyfikacji.

1. ~~**Liczba wierszy `scrape_targets`.**~~ **ROZSTRZYGNIĘTE 2026-09-04: 23 wiersze** (5 + 7 + 6 + 5). Wcześniejsze „21"/„22" było błędem arytmetycznym w artefaktach, nie brakującym URL-em; `approved-urls.md`, `scope-clarifications.md` i `requirements.md` zostały poprawione. Lista URL-i jest kompletna.
2. **Mechanika listingu NoFluffJobs i solid.jobs.** `requirements.md` F2 jawnie odkłada to „do ustalenia przy implementacji". Nieznane: infinite scroll czy paginacja, selektor karty oferty, sposób budowania URL-a kolejnej strony. Dotyczy również Bulldogjoba — kształt URL-a strony 2 i 3 nie został zweryfikowany.
3. **Selektory gotowości (`ReadySelector`) dla czterech portali.** Zależą od DOM-u i będą znane dopiero przy implementacji, razem z poz. 2. Dla justjoin.it naturalnym kandydatem jest `h1`, którego dzisiejszy kod używa jako niejawnego sygnału gotowości.
4. ~~**Czy podnieść `listingTimeout`.**~~ **ROZSTRZYGNIĘTE 2026-09-04 przez M5:** pytanie straciło sens w dotychczasowej formie. Skoro implementacja `LinkCollector` sama otwiera zakładki, budżet czasu należy do jednostki pracy (jedna strona paginacji albo jeden przewijany listing), a nie do całego targetu. Globalna stała `listingTimeout` znika, każda implementacja ustala własny budżet — patrz [9.6](#96-throttling-i-limity).
5. **Recykling instancji Chromium co N ofert w transformerze.** `gap-analysis.md` 1.5 podniósł problem (jedna instancja żyjąca ~4 h przy nowym throttlingu), ale decyzja nie zapadła. Alternatywa: pozostawić jedną instancję na batch i polegać na `mem_limit` plus `restart: always`.
6. **Rozmiar puli `pgxpool`.** `internal/postgres/client.go` nie konfiguruje `max_conns` jawnie; domyślnie `max(4, NumCPU)`. Cztery pipeline'y × (lookup w transformerze + zapis w persist) mogą wysycić pulę na małym VPS-ie. `gap-analysis.md` 4.1 zaleca sprawdzenie; wartość nie została ustalona.
7. ~~**Brzmienie i lokalizacja stałej placeholdera.**~~ **ROZSTRZYGNIĘTE 2026-09-04:** `internal/offer.Unparsed = "(unparsed)"` — nowy minimalny pakiet `internal/offer` z jedną stałą, konieczny by uniknąć zależności `internal/handlers` → `internal/justjoin` (wciągnęłaby `chromedp` do binarki `cmd/web`).
8. ~~**Czy backfill `possibilities.source_id` wchodzi w to wdrożenie.**~~ **ROZSTRZYGNIĘTE 2026-09-04: tak, w tym samym wdrożeniu.** Krok 6 runbooka wykonuje się teraz — idempotentny `UPDATE` z guardem `url LIKE 'https://justjoin.it/%'`. Kolumna `Source` jest sensowna od pierwszego dnia, bez okresu przejściowego z `-` dla całej historycznej bazy.
9. ~~**Nazwy slugów w `sources`.**~~ **ROZSTRZYGNIĘTE 2026-09-04:** `justjoin`, `nofluffjobs`, `bulldogjob`, `solidjobs` — bez kropek, wygodne jako klucze map `map[string]LinkCollector` / `map[string]OfferParser` w `main()`. Nazwy wyświetlane pozostają pełne (`justjoin.it`, `nofluffjobs.com`, `bulldogjob.pl`, `solid.jobs`) i to one trafiają do kolumny `Source`. Uwaga utrzymaniowa bez zmian: slug żyje w bazie i w kodzie, więc jego zmiana wymaga `UPDATE` **i** rekompilacji.


---

## Aneks A — rozstrzygnięcia po audycie specyfikacji

Audyt (`verification/spec-audit.md`, werdykt `pass-with-concerns`: 0 krytycznych, 5 wysokich, 7 średnich, 7 niskich) potwierdził 21 z 24 cytatów co do numeru wiersza. Poniżej zmiany naniesione na tę specyfikację 2026-09-04, wszystkie zatwierdzone przez użytkownika.

| Ustalenie | Rozstrzygnięcie | Sekcja |
|---|---|---|
| **H1** — katalog migracji nie jest montowany do kontenera, `psql -f` go nie znajdzie | Pliki czytane na hoście i podawane potokiem na stdin, tak jak dzisiejszy heredoc `deploy.sh:55`. Zero zmian w montowaniach | [7.1](#71-pliki) |
| **H2** — kroki danych z runbooka nie miały gdzie się wykonać | Osobne pliki `internal/database/data/*.sql`, podawane tym samym potokiem. „Separate Schema and Data" faktycznie spełnione, `deploy.sh` pozostaje nieinteraktywny | [7.1](#71-pliki), [7.3](#73-kolejność-kroków-przy-deployu-na-żywą-bazę) |
| **H3** — kryterium 14 dawałoby fałszywy alarm po backfillu | Sprawdzenie odcięte po `created_at` względem momentu seedu, zamiast po `source_id IS NOT NULL` | [14](#14-kryteria-akceptacji) poz. 14 |
| **H4** — wiersze `(unparsed)` byłyby nieodwracalne | `unseen` je pomija, a `ON CONFLICT DO UPDATE` nadpisuje treść **wyłącznie** gdy wiersz w bazie jest placeholderem. Dla poprawnie sparsowanych wierszy zachowanie identyczne jak dziś | [5.3](#53-czego-nie-wolno-ruszyć) |
| **H5** — `knownOffersTimeout` zostawał na 5 s | Podniesiony do 30 s. Przekroczenie jest kosztowniejsze niż przy zapisie: cały batch wraca jako nieznany i crawler scrapuje po raz drugi to, co już ma | [9.6](#96-throttling-i-limity) |
| **M1** — trzecie miejsce łamania kompilacji | `internal/router/routing_test.go:118` dopisane. `go build` tego nie wykryje — potrzebne `go test ./...` | [8.3](#83-regeneracja-i-wpływ-na-querier) |
| **M2** — fałszywa przesłanka o `/dev/shm` | `disable-dev-shm-usage` jest już w `DefaultExecAllocatorOptions` (`chromedp@v0.16.0/allocate.go:69`). Propozycja usunięta jako martwy kod; zostaje samo `mem_limit: 4g` | [7.2](#72-zmiany-w-docker-composeyaml) |
| **M3** — kryterium 24 nie wykrywało zależności tranzytywnej | `grep` zastąpiony przez `go list -deps ./cmd/web` | [14](#14-kryteria-akceptacji) poz. 24 |
| **M5** — spec nie mówił, kto otwiera zakładkę | Implementacja `LinkCollector` dostaje kontekst przeglądarki i sama zarządza zakładkami. W konsekwencji `listingTimeout` przestaje być globalną stałą — to zamyka kwestię otwartą nr 4 | [9.1](#91-dwa-wąskie-interfejsy), [9.6](#96-throttling-i-limity) |
| **M7** — zagłodzenie targetów przy stałej kolejności | Rotacja kolejności targetów w obrębie portalu między przebiegami, bezstanowa (`N mod len(targets)`) | [9.6](#96-throttling-i-limity) |
| **R1** (z planowania) — `unseen` nie mógł pomijać placeholderów, bo widzi tylko URL-e | `ExistingPossibilityUrls` dostaje `AND job_title <> '(unparsed)'`. Jedyne odstępstwo od „dedup nietknięty", świadomie zatwierdzone — bez niego wiersze `(unparsed)` są nieodwracalne | [5.3](#53-czego-nie-wolno-ruszyć) |
| **R2** (z planowania) — kryterium 5 było niespełnialne wobec H4 | Kryterium zawężone do celu konfliktu i zachowania dla wiersza poprawnie sparsowanego, zamiast dosłownej niezmienności linii `SET` | [14](#14-kryteria-akceptacji) poz. 5 |

**Poprawione błędy cytowania:** `Makefile:71` → `Makefile:55`; `docker-compose.yaml:51` → `:54`.

**Liczba wierszy seedu.** Pierwotne „21" było błędem arytmetycznym (wypisane były 22 wiersze). Przejście 22 → 23 to natomiast **zatwierdzone rozszerzenie zakresu** o `bulldogjob skills,C`, a nie kolejna pomyłka rachunkowa. Artefakty `approved-urls.md`, `scope-clarifications.md` i `requirements.md` zostały doprowadzone do 23.

**Świadome długi potwierdzone przez audyt jako poprawnie odnotowane, nie jako luki:** brak testu integracyjnego przeciw Postgresowi, brak transakcji przy zapisie oferty i HTML-a, `possibility_html` write-only bez retencji, brak `CRAWLER_*`, brak ewidencji migracji, deduplikacja semantyczna poza zakresem.
