# Zatwierdzone URL-e listingów (seed tabeli `scrape_targets`)

Data zatwierdzenia: 2026-09-04
Zatwierdził: użytkownik (twardy gate przed implementacją)
Metoda weryfikacji: każdy adres odpytany realną przeglądarką (Playwright); sprawdzono kod odpowiedzi, `<title>`, `rel=canonical` oraz liczbę linków do ofert w zwróconym HTML.

Kategorie odpowiadają `category.All` w `internal/category/category.go`: `go`, `php`, `architect`, `ai-ml`, `c`.

---

## justjoin.it (istniejące, bez zmian)

`scrapeParams = "?orderBy=DESC&sortBy=newest"`

| Kategoria | URL |
|---|---|
| go | `https://justjoin.it/job-offers/all-locations/go?orderBy=DESC&sortBy=newest` |
| php | `https://justjoin.it/job-offers/all-locations/php?orderBy=DESC&sortBy=newest` |
| architect | `https://justjoin.it/job-offers/all-locations/architecture?orderBy=DESC&sortBy=newest` |
| ai-ml | `https://justjoin.it/job-offers/all-locations/ai?orderBy=DESC&sortBy=newest` |
| c | `https://justjoin.it/job-offers/all-locations/c?orderBy=DESC&sortBy=newest` |

## nofluffjobs.com

Oferty pod `/pl/job/<slug>`. Parametr `?sort=newest` **zweryfikowany** — realnie zmienia kolejność wyników.
Decyzja użytkownika: wiele URL-i na kategorię tam, gdzie portal rozbija technologię na osobne strony.

| Kategoria | URL |
|---|---|
| go | `https://nofluffjobs.com/pl/go?sort=newest` |
| go | `https://nofluffjobs.com/pl/golang?sort=newest` |
| php | `https://nofluffjobs.com/pl/php?sort=newest` |
| architect | `https://nofluffjobs.com/pl/architecture?sort=newest` |
| ai-ml | `https://nofluffjobs.com/pl/artificial-intelligence?sort=newest` |
| c | `https://nofluffjobs.com/pl/c?sort=newest` |
| c | `https://nofluffjobs.com/pl/c++?sort=newest` |

Odrzucone warianty (sprawdzone, nie istnieją — zwracają generyczny tytuł portalu): `/pl/architektura`, `/pl/ai-ml`, `/pl/machine-learning`, `/pl/sztuczna-inteligencja`, `/pl/cpp`.
`/pl/ai` istnieje, ale ma zepsuty tytuł (`Praca search-filters.ai`) — odrzucone na rzecz `/pl/artificial-intelligence`, którego tytuł to dosłownie „Praca AI/ML".

**Uwaga implementacyjna:** URL `/pl/c++` zawiera dosłowne `+` w ścieżce i tak właśnie działa (weryfikowane). Nie kodować jako `%2B%2B`.

## bulldogjob.pl

Oferty pod `/companies/jobs/<id>-<slug>`. Filtry łączy się slashem, np. `s/role,architect/skills,Go`.
Decyzja użytkownika: dla `architect` i `ai-ml` filtr **roli**, nie umiejętności.

| Kategoria | URL | Ofert w HTML przy weryfikacji |
|---|---|---|
| go | `https://bulldogjob.pl/companies/jobs/s/skills,Go` | 12 |
| php | `https://bulldogjob.pl/companies/jobs/s/skills,PHP` | 21 |
| architect | `https://bulldogjob.pl/companies/jobs/s/role,architect` | 50 (limit strony) |
| ai-ml | `https://bulldogjob.pl/companies/jobs/s/role,ai` | 50 (limit strony) |
| c | `https://bulldogjob.pl/companies/jobs/s/skills,C++` | 17 |
| c | `https://bulldogjob.pl/companies/jobs/s/skills,C` | 6 |

Odrzucone: `skills,Golang` (0 ofert — zła nazwa umiejętności), `skills,Machine Learning` i `skills,Artificial Intelligence` (0 ofert).
Warianty węższe, świadomie niewybrane: `skills,Architecture` (5 ofert), `skills,AI` (38 ofert).

**Zatwierdzone w Fazie 2 jako rozszerzenie zakresu:** `https://bulldogjob.pl/companies/jobs/s/skills,C` — osobny zbiór 6 ofert, nieobjęty przez `skills,C++`. Analogia do decyzji o NoFluffJobs (`/pl/c` + `/pl/c++`). To szósty wiersz bulldogjob.pl i dwudziesty trzeci w całym seedzie.

## solid.jobs

Oferty pod `/offer/<id>/<slug>`. Portal sam przekierowuje na adres kanoniczny.

| Kategoria | URL |
|---|---|
| go | `https://solid.jobs/praca/golang-developer` |
| php | `https://solid.jobs/praca/php-developer` |
| architect | `https://solid.jobs/praca/architekt` |
| ai-ml | `https://solid.jobs/praca/data-science` |
| c | `https://solid.jobs/praca/c-cpp-developer` |

`/praca/c-cpp-developer` pokrywa C **i** C++ jedną stroną — brak potrzeby rozbijania.
`ai-ml`: solid.jobs nie ma dedykowanego landingu AI/ML (`/praca/ai`, `/praca/machine-learning` nie istnieją). Użytkownik wybrał `/praca/data-science` jako najbliższy tematycznie, świadomie akceptując, że wpadną też oferty czysto danowe (Data Engineer, BI), które nie są AI/ML.

---

## Podsumowanie

**23 wiersze w `scrape_targets`**: justjoin.it 5, nofluffjobs.com 7, bulldogjob.pl 6, solid.jobs 5.

`bulldogjob skills,C` zostal zatwierdzony w Fazie 2 jako dodatkowy wiersz, stad 6 dla bulldogjob.pl.

## Kwestia otwarta: sortowanie „najnowsze pierwsze"

| Portal | Status |
|---|---|
| justjoin.it | `?orderBy=DESC&sortBy=newest` — działa dziś |
| nofluffjobs.com | `?sort=newest` — **zweryfikowane**, zmienia kolejność |
| bulldogjob.pl | **nierozwiązane** — `sort,newest` jest przyjmowane, ale nie zmienia wyników; `order,published` → HTTP 500 |
| solid.jobs | **nierozwiązane** — nie znaleziono parametru sortowania |

Obecny fetcher polega na sortowaniu po stronie portalu i odwraca listę przez `slices.Reverse` (`fetcher.go:110`), żeby zapisywać od najstarszych. Dla Bulldogjob i solid.jobs to założenie **nie jest spełnione** — ekstraktory tych portali muszą ustalić kolejność inaczej (np. z daty oferty) albo świadomie z niej zrezygnować. Do rozstrzygnięcia w specyfikacji.
