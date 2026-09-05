# Doprecyzowania — Faza 1

Data: 2026-09-04
Źródło: AskUserQuestion po analizie kodu (`analysis/codebase-analysis.md`).

## 1. Model źródeł (portali) i URL-i w bazie

**Decyzja: `sources` + `scrape_targets` + FK w `possibilities`.**

- `sources` — rejestr portali (slug, nazwa).
- `scrape_targets` — para portal × kategoria z URL-em listingu i flagą `enabled`, dzięki czemu pojedyncze kombinacje można wyłączać bez rekompilacji.
- `possibilities.source_id BIGINT REFERENCES sources` — indeksowany klucz obcy.

Uzasadnienie: zgodne z `standards/backend/models.md` („Database Constraints", „Index Foreign Keys", „Clear Relationships"). Kosztem większej liczby zmian dostajemy czysty model i sterowanie zakresem crawlowania z bazy.

**Konsekwencja:** `internal/category/category.go` przestaje być jedynym źródłem prawdy o URL-ach. Cztery miejsca konsumujące `category.All` / `category.BySlug` (`cmd/crawler/main.go`, `internal/templates/parse.go`, `internal/handlers/helper.go`, `internal/handlers/stats_handler.go`) muszą zostać przepięte lub zachować dotychczasową semantykę przy zasilaniu z bazy.

## 2. Miejsce składowania surowego HTML

**Decyzja: osobna tabela 1:1.**

Kształt: `possibility_html(possibility_id PK/FK → possibilities, html TEXT, fetched_at TIMESTAMPTZ)`.

Uzasadnienie: `possibilities` zostaje szczupłe, więc lista ofert (`ListPossibilitiesByCategory`) i statystyki (`StatsPossibilityByCategory`) nie ciągną blobów. HTML da się kasować niezależnie od metadanych oferty.

**Konsekwencja:** zapis oferty przestaje być pojedynczym `INSERT` — potrzebna transakcja (`WithTx` w `internal/database/generated/db.go` już istnieje i jest dziś nieużywane) albo drugi zapis po `RETURNING id`. Timeout `3 * time.Second` w `internal/crawler/persist.go` jest za ciasny dla zapisu setek KB i wymaga podniesienia.

## 3. Słowa kluczowe

**Decyzja: reuse istniejącej kolumny `tech`.**

Tagi/skille z każdego portalu trafiają do istniejącej kolumny `tech TEXT` w dotychczasowym formacie (wartości rozdzielone `", "`), tak jak dziś robi to ekstraktor justjoin.it. Brak nowej kolumny `keywords`.

Uzasadnienie: zero zmian w schemacie pod tym kątem i zero zmian w widoku statystyk, który agreguje po `tech`.

**Konsekwencja / ryzyko do odnotowania w specyfikacji:** statystyki (`StatsPossibilityByCategory`) zaczną mieszać nazewnictwo technologii z 4 portali w jednym polu (np. „JavaScript" vs „JS" vs „Javascript"), co rozmyje agregacje. Normalizacja nazw jest **poza zakresem** tego zadania — surowy HTML zostaje zapisany, więc dokładniejsze wyciągnięcie słów kluczowych jest możliwe później, przy deduplikacji LLM-em.

## 4. Przepustowość przy 2-3 min na ofertę

**Decyzja: równoległość per portal.**

Cztery portale scrapowane równolegle — osobna instancja Chromium i osobny throttling na portal; pauza 2-3 min dotyczy pojedynczego portalu, nie całego przebiegu.

Uzasadnienie: skraca przebieg ~4× względem wariantu sekwencyjnego, bez wprowadzania trwałej kolejki w bazie (co byłoby zmianą architektury pipeline'u).

**Konsekwencje do rozstrzygnięcia w specyfikacji:**
- Zużycie RAM: 4 równoległe instancje Chromium zamiast jednej. Kontener crawlera (`docker/crawler/Dockerfile`, Alpine + Chromium) może potrzebować limitu pamięci w `docker-compose.yaml`.
- Graceful shutdown: obecny wzorzec (`cancel(ctx)` → fetcher zamyka `FetchedUrls` → transformer domyka `Offers` → persist dopina) musi objąć N równoległych gałęzi bez zamykania kanału dwa razy.
- Równoległość **nie ogranicza** rozmiaru kolejki: przy pierwszym uruchomieniu (backfill 4 portali od zera) przebieg nadal potrwa wiele godzin, a `FetchedUrls` ma bufor 100. Zachowanie przy przepełnieniu bufora (blokada fetchera) trzeba świadomie potwierdzić lub dołożyć limit ofert na przebieg.

## Kwestie przeniesione do dalszych faz

- **URL-e listingów dla NoFluffJobs, Bulldogjob i solid.jobs** — użytkownik zatwierdza je ręcznie. Twardy gate przed implementacją (Faza 5/7).
- **Sposób dostarczenia zmian schematu na żywą bazę** — dziś ręcznie nadpisywany heredoc w `scripts/deploy.sh`, niezgodny ze wszystkimi czterema regułami `standards/backend/migrations.md`. Decyzja „zostajemy przy konwencji" vs „wprowadzamy katalog migracji" → Faza 5 (technical clarifications).
- **Schemat `wok` vs `public`** — schemat `wok` nie jest tworzony nigdzie w repo; istnieje wyłącznie dzięki ręcznej operacji na VPS. Całe nowe DDL musi opierać się na `current_schema()`, nigdy na `public`.
