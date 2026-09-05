# Mockupy UI: kolumna źródła oferty (portal)

**Data:** 2026-09-04
**Zadanie:** `.maister/tasks/development/2026-09-04-crawler-multi-portal-sources`
**Typ zmiany:** enhancement (jedna kolumna w istniejącej tabeli)
**Zakres zatwierdzony:** `analysis/scope-clarifications.md` — sekcja „Decyzje ważne — zatwierdzone", punkt „Kolumna `Source` w liście ofert"

---

## 1. Przegląd {#przeglad}

### Wymaganie UI

Jedna nowa kolumna w `internal/templates/scraped/list.gohtml` pokazująca, z którego portalu pochodzi oferta.
Bez filtrowania, bez CRUD-u źródeł, bez podglądu surowego HTML-a.

### Strategia integracji

**Decyzja:** kolumna `Source` jako **druga** kolumna tabeli, między `Title` a `Published`, renderowana jako neutralny badge (`badge text-bg-light border`) z pełną nazwą portalu z `sources.name`.

**Uzasadnienie:**
- Ta sama oferta może wystąpić w kilku wierszach z różnych portali (deduplikacja poza zakresem). Gdy `Source` sąsiaduje z `Title`, powtórzony tytuł i różniący się portal są w jednym rzucie oka. Gdyby `Source` był na końcu, porównanie wymagałoby przeskoku wzroku przez długą kolumnę `Technologies`.
- Nazwa portalu jest krótka (max `nofluffjobs.com`, 15 znaków) i stała długościowo — nie rozpycha układu.
- Badge `text-bg-light border` to **istniejący** wzorzec z `internal/templates/offers/list.gohtml:40,44` (gałąź fallback dla `Status` i `HadChat`). Zero nowego CSS, zero nowych kolorów semantycznych.

---

## 2. Analiza istniejącego układu {#analiza-ukladu}

### Struktura strony

Wszystkie ekrany składane są z tych samych partiali (`internal/templates/parse.go`, `embed.go`):

| Warstwa | Plik | Rola |
|---|---|---|
| Head + otwarcie `<main class="container pb-5">` | `internal/templates/partials/head.gohtml` | Bootstrap 5 z `/assets/bootstrap.min.css`, brak JS |
| Górna nawigacja (brand + kategorie + Logout) | `internal/templates/partials/header.gohtml` | Linki `/scr4p3d/{{ .Slug }}` z funkcji `categories` |
| Nagłówek kategorii + przełącznik Scraped/Stats | `internal/templates/partials/categorynav.gohtml` | Przyjmuje `.Category` i `.Page` |
| Treść | `internal/templates/scraped/list.gohtml` | `card shadow-sm` → `table-responsive` → `table table-striped table-hover align-middle mb-0` |
| Zamknięcie | `internal/templates/partials/footer.gohtml` | — |

### Zidentyfikowane konwencje

1. **Jedna tabela na ekran** w `card shadow-sm` + `table-responsive` — identycznie w `scraped/list.gohtml`, `stats/stats.gohtml`, `offers/list.gohtml`.
2. **`thead class="table-light"`**, `<th>` bez klas poza `text-end` dla liczb.
3. **Pierwsza kolumna `<td class="fw-medium">`** — nośnik tożsamości wiersza.
4. **`<td class="text-nowrap">` dla dat** (`scraped/list.gohtml:23`).
5. **Badge dla wartości słownikowych** — `offers/list.gohtml:36-45`, z gałęzią `{{ else }}` → `badge text-bg-light border`.
6. **Pusty stan przez `{{ else }}` w `range`** z `colspan` równym liczbie kolumn (`scraped/list.gohtml:29`, `stats/stats.gohtml:25`, `offers/list.gohtml:53`).
7. **Formatowanie wartości pustych w Go, nie w szablonie** — `formatDate`/`formatText` w `internal/handlers/helper.go:36,64` zwracają `-`.

---

## 3. Mockup: stan obecny (BEFORE) {#before}

```
┌────────────────────────────────────────────────────────────────────────────────────────────┐
│ WOK      [Go] [PHP] [Software Architect] [AI/ML] [C/C++]                        [Logout]   │ partials/header.gohtml
└────────────────────────────────────────────────────────────────────────────────────────────┘

  Go                                                    [ Scraped offers ][ Stats ]            partials/categorynav.gohtml
                                                          ^ btn-secondary  ^ btn-outline

┌────────────────────────────────────────────────────────────────────────────────────────────┐ card shadow-sm
│ ┌────────────────────────┬────────────┬───────────────────┬──────────────────────────────┐ │ table-responsive
│ │ Title                  │ Published  │ Technologies      │ URL                          │ │ thead.table-light
│ ├────────────────────────┼────────────┼───────────────────┼──────────────────────────────┤ │
│ │ Senior Go Developer    │ 2026-09-03 │ Go, Kubernetes    │ https://justjoin.it/job-of…  │ │
│ │ Backend Engineer (Go)  │ 2026-09-02 │ Go, PostgreSQL    │ https://justjoin.it/job-of…  │ │
│ │ Golang Developer       │ 2026-09-02 │ Go, gRPC, Docker  │ https://justjoin.it/job-of…  │ │
│ └────────────────────────┴────────────┴───────────────────┴──────────────────────────────┘ │
└────────────────────────────────────────────────────────────────────────────────────────────┘

  scraped/list.gohtml:13-16  <th> ×4
  scraped/list.gohtml:22-25  <td> ×4  ← possibilityRow{JobTitle, Published, Tech, Url}
  scraped/list.gohtml:29     colspan="4"
```

**Stan wyjściowy:** jeden portal, więc domena w kolumnie `URL` jest zawsze ta sama i nie niesie informacji.

---

## 4. Mockup: stan docelowy (AFTER) {#after}

```
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ WOK      [Go] [PHP] [Software Architect] [AI/ML] [C/C++]                                        [Logout]   │ BEZ ZMIAN
└────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

  Go                                                                    [ Scraped offers ][ Stats ]           BEZ ZMIAN
                                                                                                              (patrz §7)

┌────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ ┌────────────────────────┬───────────────────┬────────────┬───────────────────┬──────────────────────────┐ │
│ │ Title                  │ Source      NOWE  │ Published  │ Technologies      │ URL                      │ │
│ ├────────────────────────┼───────────────────┼────────────┼───────────────────┼──────────────────────────┤ │
│ │ Senior Go Developer    │ (justjoin.it)     │ 2026-09-03 │ Go, Kubernetes    │ https://justjoin.it/j…   │ │
│ │ Go Engineer            │ (nofluffjobs.com) │ 2026-09-03 │ Go, AWS, Terraform│ https://nofluffjobs.c…   │ │
│ │ Backend Developer Go   │ (bulldogjob.pl)   │ -          │ Go, Docker        │ https://bulldogjob.pl…   │ │
│ │ Golang Developer       │ (solid.jobs)      │ -          │ Go                │ https://solid.jobs/of…   │ │
│ └────────────────────────┴───────────────────┴────────────┴───────────────────┴──────────────────────────┘ │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
                             ^^^^^^^^^^^^^^^^^
                             ( ... ) = <span class="badge text-bg-light border">

Adnotacje:
  <th>Source</th>                              → scraped/list.gohtml, nowa linia po :13 (po <th>Title</th>)
  <td class="text-nowrap">                     → scraped/list.gohtml, nowa linia po :22
    <span class="badge text-bg-light border">
      {{ .Source }}
    </span>
  colspan="4" → colspan="5"                    → scraped/list.gohtml:29  ← ŁATWE DO PRZEOCZENIA
  possibilityRow.Source string                 → internal/handlers/scraped_handler.go:28-33
  mapowanie source_id → sources.name           → internal/handlers/scraped_handler.go:62-70
```

**Uwaga o szerokości:** tabela rośnie z 4 do 5 kolumn. Nie potrzeba nowego mechanizmu — `table-responsive` (`scraped/list.gohtml:9`) już daje poziomy scroll na wąskich ekranach. `Technologies` pozostaje jedyną kolumną zawijającą się; `Source` dostaje `text-nowrap`, bo `nofluffjobs.com` nie powinien się łamać.

---

## 5. Warianty wierszy {#warianty-wierszy}

### 5.1 Wiersz normalny {#wiersz-normalny}

```
│ Senior Go Developer    │ (justjoin.it)     │ 2026-09-03 │ Go, Kubernetes    │ https://justjoin.it/j… │
  fw-medium                badge text-bg-light  text-nowrap
  .JobTitle                .Source              .Published   .Tech               .Url
```

### 5.2 Wiersz bez daty publikacji {#wiersz-bez-daty}

`possibilities.posted_at` jest nullable; Bulldogjob i solid.jobs nie zawsze podają datę.

```
│ Backend Developer Go   │ (bulldogjob.pl)   │ -          │ Go, Docker        │ https://bulldogjob.p… │
                                               ^^^
                                               formatDate(pgtype.Date{Valid:false}) → "-"
```

**Zmiana wymagana: ŻADNA.** `internal/handlers/helper.go:36-41` już zwraca `-` dla nieustawionej daty, a `formatText` (`helper.go:64-69`) robi to samo dla pustego `tech`. Myślnik jest istniejącą konwencją pustej wartości w całej aplikacji (`offers/list.gohtml` przez `formatPrice`).

**Konsekwencja sortowania (do odnotowania, nie do naprawiania):** `ListPossibilitiesByCategory` (`internal/database/queries/possibilities.sql:4-7`) sortuje po `created_at DESC, id DESC`, czyli po momencie zescrapowania, a nie po `posted_at`. Kolumna `Published` była już wcześniej niemonotoniczna; przy czterech portalach o różnych porządkach listingu (`slices.Reverse` włączone tylko dla justjoin.it i NoFluffJobs) będzie po prostu bardziej wymieszana. Kolumna `Source` czyni tę niemonotoniczność **zrozumiałą** — użytkownik widzi, że skok daty pokrywa się ze zmianą portalu.

### 5.3 Duplikat: ta sama oferta z dwóch portali {#wiersz-duplikat}

Deduplikacja semantyczna jest poza zakresem — użytkownik robi ją ręcznie poza aplikacją. UI ma tylko **nie utrudniać** rozpoznania duplikatu.

```
│ Senior Golang Engineer │ (justjoin.it)     │ 2026-09-03 │ Go, K8s, AWS      │ https://justjoin.it/j… │
│ Senior Golang Engineer │ (nofluffjobs.com) │ 2026-09-03 │ Go, Kubernetes    │ https://nofluffjobs.c… │
  └─ identyczny tytuł ─┘   └─ różny portal ─┘              └ różny zapis tech ┘
     w dwóch sąsiednich       jedyny czytelny
     wierszach                dyskryminator
```

Właśnie ten wariant przesądza o **pozycji kolumny**: `Source` bezpośrednio przy `Title`, żeby powtórzony tytuł i różniący się portal leżały obok siebie. `table-striped` dodatkowo pomaga — sąsiadujące duplikaty mają różne tło.

**Ograniczenie do zaakceptowania:** wiersze duplikatów **nie są gwarantowanie sąsiednie**. Porządek to `created_at DESC`, a cztery pipeline'y działają równolegle, więc duplikaty z jednego przebiegu wylądują blisko siebie, ale niekoniecznie obok. Grupowanie po tytule wymagałoby zmiany zapytania — poza zakresem.

### 5.4 Wiersz z placeholderem po nieudanym parsowaniu {#wiersz-placeholder}

Decyzja `failed-parse-policy` (`scope-clarifications.md:17`): oferta trafia do bazy z surowym HTML-em i rozpoznawalnym placeholderem w `job_title`/`company_name`.

```
│ (unparsed)             │ (bulldogjob.pl)   │ -          │ -                 │ https://bulldogjob.p… │
  ^^^^^^^^^^                                   ^            ^
  text-muted fst-italic                        formatDate   formatText
  zamiast fw-medium                            → "-"        → "-"
```

**Odróżnienie wizualne bez nowego mechanizmu:** zamiana `fw-medium` na `text-muted fst-italic` w komórce tytułu. Obie klasy to standardowe utilities Bootstrapa, a `text-muted` jest już używane w aplikacji dla treści drugorzędnej (`scraped/list.gohtml:29`, `stats/stats.gohtml:25`). Zero nowego CSS, zero nowych partiali, zero JS.

Wiersz jest rozpoznawalny również „za darmo": placeholder w `Title` plus `-` w `Published` i `Technologies` daje charakterystyczną, pustą sylwetkę wiersza. Wyszarzenie tytułu tylko to wzmacnia.

**Skąd flaga.** Dwie opcje dla planisty:

| Wariant | Realizacja | Ocena |
|---|---|---|
| A — porównanie w szablonie | `{{ if eq .JobTitle "(unparsed)" }}` — dokładnie wzorzec z `offers/list.gohtml:36-40` | Zero zmian w `possibilityRow`, mieści się w zatwierdzonym zakresie. Wada: literał sentinela zduplikowany między crawlerem a szablonem — kolizja z DRY (`standards/global/coding-style.md`) |
| B — flaga w view modelu | `possibilityRow.Unparsed bool`, ustawiane w handlerze przez porównanie ze **wspólną, eksportowaną stałą** crawlera | Sentinel w jednym miejscu. Wada: **drugie** nowe pole w `possibilityRow`, o jedno więcej niż zatwierdzony zakres |

**Rekomendacja: B**, z jawnym potwierdzeniem rozszerzenia zakresu o jedno pole. Jeśli potwierdzenie nie przyjdzie, A jest w pełni akceptowalne i zgodne z istniejącą konwencją szablonów.

### 5.5 Pusty stan {#pusty-stan}

```
┌────────────────────────┬───────────────────┬────────────┬───────────────────┬──────────────────────┐
│ Title                  │ Source            │ Published  │ Technologies      │ URL                  │
├────────────────────────┴───────────────────┴────────────┴───────────────────┴──────────────────────┤
│                                    No scraped offers yet.                                          │
│                              colspan="5"  text-center text-muted py-5                              │
└────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

`scraped/list.gohtml:29` — `colspan="4"` → `colspan="5"`. Bez tej zmiany komunikat pustego stanu przestaje być wyśrodkowany względem tabeli.

---

## 6. Skala listy po zmianie {#skala-listy}

```
DZIŚ                              PO ZMIANIE
1 portal × N ofert                4 portale × ~N ofert   (+ duplikaty między portalami)
                                  ≈ 4× dłuższa lista, bez paginacji

┌──────────────────────────────┐  ┌──────────────────────────────┐
│ Title │ Publ │ Tech │ URL    │  │ Title │ Src │ Publ │Tech│URL │
├───────┼──────┼──────┼────────┤  ├───────┼─────┼──────┼────┼────┤
│  ~150 wierszy                │  │  ~600 wierszy                │
│  jeden <table>               │  │  wciąż jeden <table>         │
└──────────────────────────────┘  └──────────────────────────────┘
```

**Zmiana wymagana: ŻADNA.** Paginacja, wirtualizacja i filtrowanie po źródle są poza zatwierdzonym zakresem. Aplikacja ma jednego użytkownika, render jest po stronie serwera, a `table-responsive` i `table-hover` już działają przy dowolnej liczbie wierszy. Odnotowane jako świadomie przyjęty dług — kolumna `Source` niczego tu nie pogarsza, przeciwnie: bez niej lista 600 wierszy z czterech portali byłaby nieczytelna.

---

## 7. Ocena `categorynav.gohtml` {#categorynav}

```
┌────────────────────────────────────────────────────────────────────────┐
│  Go                                    [ Scraped offers ][ Stats ]     │
│  └─ .Category.Name                     └─ /scr4p3d/{{ .Category.Slug }}│
│                                        └─ /st4ts/{{ .Category.Slug }}  │
└────────────────────────────────────────────────────────────────────────┘
```

**Werdykt: BEZ ZMIAN.**

Uzasadnienie:
- Partial operuje wyłącznie na `.Category` i `.Page`. Obie wartości pochodzą z `category.All`, którego kształt po zmianie traci jedynie pole `ScrapeURL` (decyzja `category-source-of-truth`) — pola `Slug` i `Name`, jedyne używane w tym pliku, zostają nietknięte.
- Nie ma tu ani jednej referencji do portalu ani do `possibilities`.
- Naturalnym powodem do zmiany byłby przełącznik źródeł obok Scraped/Stats — a filtrowanie po źródle jest jawnie **poza zakresem** (`scope-clarifications.md:33`).

To samo dotyczy `partials/header.gohtml`, `partials/head.gohtml`, `partials/footer.gohtml` i `stats/stats.gohtml` — żaden nie wymaga dotknięcia.

---

## 8. Ponowne użycie a elementy nowe {#reuse}

### Wykorzystane ponownie, bez modyfikacji

| Element | Plik | Zastosowanie |
|---|---|---|
| `head` | `internal/templates/partials/head.gohtml` | Bootstrap, brak build pipeline'u |
| `header` | `internal/templates/partials/header.gohtml` | Nawigacja globalna |
| `categorynav` | `internal/templates/partials/categorynav.gohtml` | Nagłówek kategorii — patrz §7 |
| `footer` | `internal/templates/partials/footer.gohtml` | — |
| `card shadow-sm` + `table-responsive` + `table table-striped table-hover align-middle mb-0` | `internal/templates/scraped/list.gohtml:8-10` | Kontener tabeli, obsługa przepełnienia poziomego |
| `badge text-bg-light border` | wzorzec z `internal/templates/offers/list.gohtml:40,44` | Renderowanie nazwy portalu |
| `text-nowrap` | wzorzec z `internal/templates/scraped/list.gohtml:23` | Komórka `Source` |
| `text-muted` | wzorzec z `internal/templates/scraped/list.gohtml:29` | Wyszarzenie wiersza placeholdera |
| `formatDate` / `formatText` | `internal/handlers/helper.go:36,64` | `-` dla wartości pustych — bez zmian |
| `{{ if eq … }}` w `<td>` | wzorzec z `internal/templates/offers/list.gohtml:36-45` | Warunkowe klasy komórki tytułu |

### Nowe

| Element | Plik | Zakres zmiany |
|---|---|---|
| `<th>Source</th>` | `internal/templates/scraped/list.gohtml`, po :13 | 1 linia |
| `<td>` z badge'em źródła | `internal/templates/scraped/list.gohtml`, po :22 | 3 linie |
| `colspan="4"` → `"5"` | `internal/templates/scraped/list.gohtml:29` | 1 znak |
| `possibilityRow.Source string` | `internal/handlers/scraped_handler.go:28-33` | 1 pole |
| Mapowanie `source_id` → nazwa portalu | `internal/handlers/scraped_handler.go:62-70` | pętla mapująca |
| *(opcjonalnie, §5.4 wariant B)* `possibilityRow.Unparsed bool` | `internal/handlers/scraped_handler.go:28-33` | 1 pole |

**Nowe komponenty CSS/JS: zero.** Nowe partiale: zero.

### Uwaga implementacyjna do warstwy danych

`ListPossibilitiesByCategory` (`internal/database/queries/possibilities.sql:4-7`) zwraca `SELECT *` z `possibilities`, czyli po migracji także `source_id` (nullable, `BIGINT`). Handler potrzebuje nazwy, nie identyfikatora. Dwie drogi — decyzja należy do planisty, mockup jest wobec niej neutralny, bo obie dają identyczny render:
- **JOIN w zapytaniu** — `LEFT JOIN sources` i zwracanie `sources.name`; wymaga zmiany zapytania i regeneracji `sqlc`, ale eliminuje mapowanie w Go.
- **Mapa w handlerze** — jednorazowy odczyt `sources` do `map[int64]string`; 4 wiersze słownikowe.

Wartość dla `source_id IS NULL` (istniejące wiersze justjoin.it przed backfillem — `gap-analysis.md:48`) musi wyrenderować się jako `-`, spójnie z resztą pustych wartości. **To jest realny stan produkcyjny**, nie przypadek brzegowy: backfill jest osobnym krokiem danych, więc między migracją a backfillem cała historyczna lista pokaże `-` w kolumnie `Source`:

```
│ Senior Go Developer    │ -                 │ 2026-09-03 │ Go, Kubernetes    │ https://justjoin.it/j… │
                           ^ source_id IS NULL — wiersz sprzed backfillu, bez badge'a
```

---

## 9. Dostępność i responsywność {#a11y}

- **Semantyka.** Kolumna to zwykły `<th scope="col">`/`<td>` w istniejącej tabeli — czytnik ekranu zapowie nagłówek `Source` przy każdej komórce bez dodatkowej pracy. Aktualny szablon nie ustawia `scope` na żadnym `<th>`; dodanie go tylko do nowej kolumny byłoby niespójne, więc pozostawiamy jak jest.
- **Kolor nie niesie informacji.** Badge jest neutralny (`text-bg-light border`), a nazwa portalu to tekst — czytelne przy daltonizmie i w trybie wysokiego kontrastu.
- **Placeholder.** `text-muted fst-italic` to wskazówka wizualna, ale sam tekst `(unparsed)` niesie pełną informację — brak zależności wyłącznie od stylu.
- **Wąski ekran.** `table-responsive` przewija tabelę poziomo. Pięć kolumn, z których `Source` ma `text-nowrap` i ≤15 znaków, nie wywoła poziomego scrolla `<body>`.

---

## 10. Rozważone i odrzucone {#odrzucone}

| Wariant | Werdykt | Powód |
|---|---|---|
| Kolumna `Source` na końcu tabeli (po `URL`) | **odrzucony** | Duplikaty (§5.3) wymagałyby porównywania `Title` i `Source` rozdzielonych trzema kolumnami, w tym najszerszą |
| Kolumna `Source` między `Published` a `Technologies` | **odrzucony** | Rozbija naturalną parę „co i kiedy" bez zysku |
| Skróty zamiast pełnych nazw (`JJ`, `NFJ`, `BDJ`, `SJ`) | **odrzucony** | Oszczędza ~8 znaków kosztem legendy, której nie ma gdzie umieścić. Najdłuższa nazwa to 15 znaków — nie ma czego optymalizować |
| Favicon / logo portalu | **odrzucony** | Wymaga assetów, cache'owania i tekstu alternatywnego. Projekt nie ma build pipeline'u ani katalogu assetów poza `bootstrap.min.css` |
| Kolorowy badge per portal (`text-bg-primary` itd.) | **odrzucony** | Wprowadza semantykę koloru tam, gdzie jej nie ma — w `offers/list.gohtml` kolor koduje status (dobry/zły), a portale są równorzędne. Do tego cztery kolory bez znaczenia to szum |
| Skrócenie kolumny `URL` do samej ścieżki, skoro domena dubluje `Source` | **odrzucony w tym zadaniu** | Sensowne, ale to zmiana istniejącej kolumny poza zatwierdzonym zakresem. Do rozważenia osobno |
| Filtrowanie / sortowanie po źródle | **odrzucony** | Jawnie poza zakresem (`scope-clarifications.md:33`) |
| Grupowanie duplikatów po tytule | **odrzucony** | Deduplikacja semantyczna jest poza zakresem, robiona ręcznie poza aplikacją |
| CRUD źródeł w UI | **odrzucony** | Decyzja `sources-crud-orphan`: `psql` jest jedynym interfejsem administracyjnym |
| Podgląd surowego HTML-a z `possibility_html` | **odrzucony** | Decyzja `possibility-html-orphan`: tabela jest write-only |

---

## 11. Checklist spójności {#checklist}

- [x] Kolumna używa wyłącznie klas Bootstrapa już obecnych w `internal/templates/`
- [x] Badge odwzorowuje wzorzec z `offers/list.gohtml:40,44`
- [x] `colspan` pustego stanu podniesiony z 4 na 5 (`scraped/list.gohtml:29`)
- [x] Puste wartości renderują `-` przez istniejące `formatDate`/`formatText`, w tym `source_id IS NULL`
- [x] Zero nowych plików CSS/JS, zero nowych partiali
- [x] `categorynav.gohtml`, `header.gohtml`, `head.gohtml`, `footer.gohtml`, `stats/stats.gohtml` — bez zmian
- [x] Zmiany w Go ograniczone do `possibilityRow` i pętli mapującej w `ScrapedHandler.List`
- [ ] Do potwierdzenia: `possibilityRow.Unparsed bool` (drugie pole, §5.4 wariant B)
- [ ] Do rozstrzygnięcia przez planistę: JOIN w zapytaniu czy mapa w handlerze (§8)

---

*Wygenerowane przez subagenta ui-mockup-generator*
