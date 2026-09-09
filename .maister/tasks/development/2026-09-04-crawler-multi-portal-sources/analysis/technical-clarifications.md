# Doprecyzowania techniczne — Faza 5A

Data: 2026-09-04

## 1. Limit ofert na przebieg per portal: **100**

Wynika z odrzucenia w Fazie 2 domyślnego „backfill bez limitu".

Uzasadnienie: przy średnio 2,5 min na ofertę 100 ofert zajmuje ~4h10m, czyli mniej więcej tyle, ile trwa okno między przebiegami listingu (4-5h). Przebieg kończy się, zanim ruszy następny, więc kolejka nie narasta.

Limit jest **niezależny** od limitu 3 stron listingu dla Bulldogjob — tamten ogranicza odkrywanie URL-i, ten liczbę faktycznie scrapowanych ofert.

Konsekwencja: backfill 22 targetów rozłoży się na kilka dni. To stan przejściowy, świadomie zaakceptowany.

## 2. Timeout zapisu w `internal/justjoin/persist.go`: **60 s**

Dziś `3 * time.Second` (`persist.go:34`).

Kontekst: po odrzuceniu transakcji (`txBeginner` + `pgPool`) zapis oferty to dwa osobne `INSERT`-y — najpierw `possibilities` z `RETURNING id`, potem `possibility_html` z setkami KB. Do tego cztery równoległe pipeline'y konkurują o pulę połączeń `pgxpool`, która nie ma jawnej konfiguracji rozmiaru (`internal/postgres/client.go`).

Uzasadnienie wyboru dużego zapasu: timeout praktycznie przestaje być przyczyną utraty danych. Zablokowany zapis potrzymа pipeline minutę, ale przy pauzie 2-3 min na ofertę nie jest to wąskie gardło.

## 3. Ekstraktor per portal: **dwa wąskie interfejsy u konsumentów**

- `linkCollector` w `internal/justjoin/fetcher.go` — odkrywanie URL-i ofert z listingu (infinite scroll dla justjoin.it, paginacja dla Bulldogjob, do ustalenia dla NoFluffJobs i solid.jobs).
- `offerParser` w `internal/justjoin/transformer.go` — parsowanie pól oferty.

Zgodne z idiomem obecnym w całym repo: `knownOfferStore` (`transformer.go:28`), `OfferStore` (`persist.go:11`), `IpRepository`/`SessionRepository` (`router/routing.go:50-58`) — interfejs deklarowany u konsumenta, wyłącznie z metodami, których ten konsument używa.

Odrzucone: jeden `portalScraper` z obiema metodami (każdy etap dostawałby metodę, której nie używa) oraz struktura konfiguracyjna z selektorami (Bulldogjob ma paginację zamiast infinite scrolla, więc różnica leży w logice, nie w danych — wspólny kod rozjechałby się w `if`-y).

## 4. Rozdzielenie pobrania HTML od parsowania: **tak**

Dziś `scrapeOffer` (`transformer.go:166-225`) robi nawigację i całą ekstrakcję w jednym `chromedp.Run`, z selektorami i XPath-ami wykonywanymi w Chromium.

Docelowo dwa etapy:
1. pobranie surowego HTML przez `chromedp` (i tak potrzebne — punkt (c) zadania),
2. parsowanie w czystej funkcji Go, przyjmującej HTML jako wejście.

Uzasadnienie: skoro surowy HTML jest i tak pobierany, drugi etap jest darmowy, a parsery per portal stają się testowalne na zapisanych fixture'ach bez uruchamiania przeglądarki. Dziś `scrapeOffer` nie ma żadnego pokrycia testami.

Dotyczy **wszystkich czterech** portali, łącznie z przepisaniem istniejącej ekstrakcji justjoin.it — nie dopuszczamy dwóch równoległych stylów w jednym pakiecie.

## 5. Flaga „nieparsowalna oferta": **pole `Unparsed bool` w `possibilityRow`**

Handler porównuje `job_title` ze wspólną stałą placeholdera i ustawia flagę; szablon robi `{{ if .Unparsed }}` i renderuje komórkę tytułu w `text-muted fst-italic` zamiast `fw-medium`.

Uzasadnienie: stała placeholdera ma jedno miejsce definiujące, wspólne dla crawlera i widoku. Wariant z literałem w `.gohtml` rozjechałby się przy pierwszej zmianie jej brzmienia.

## 6. Nazwa portalu w kolumnie Source: **`LEFT JOIN sources` w zapytaniu**

`ListPossibilitiesByCategory` dostaje `LEFT JOIN sources ON possibilities.source_id = sources.id` i zwraca nazwę gotową do renderu.

Uzasadnienie: `LEFT` obsługuje wiersze sprzed backfillu (`source_id IS NULL`) bez dodatkowej logiki. Wariant z mapą w handlerze wprowadzałby cache, którego nie dałoby się odświeżyć bez restartu aplikacji — a źródła zmienia się przez `psql`.

Uwaga: zapytanie musi nadal wybierać jawną listę kolumn (`standards/backend/queries.md`: „Select Only Needed Columns") i nie może stracić zgodności z indeksem `possibilities_category_created_at_idx`.

## 7. Testy: **po kilka fixture'ów HTML na portal**

Zapisany HTML realnej oferty z każdego z czterech portali jako plik testowy plus tabelaryczny test parsera, w idiomie obecnym w repo (`transformer_test.go`, stdlib `testing`, bez testify, stuby zamiast frameworka mockującego).

Przypadki do pokrycia poza wariantem typowym: brak daty publikacji, brak stacku technologicznego, oferta zdalna, oferta nieparsowalna.

Kontekst: test integracyjny Postgresa został w Fazie 2 odrzucony, więc **to jedyne pokrycie tej zmiany**. Jest to również jedyny mechanizm, który wykryje, że któryś portal przemodelował DOM.
