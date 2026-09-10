# Kontrakt API nofluffjobs.com (crawler bez przeglądarki)

Data weryfikacji: 2026-09-05
Metoda: surowe zapytania HTTP (`curl` / `urllib`, bez JS, bez chromedp) do publicznych endpointów portalu; każdy endpoint, parametr i pułapka opisane niżej zostały odpytane na żywo, a wyniki porównane z tym, co dziś wyciąga `NofluffjobsParser` z DOM-u po renderze.
Status: ustalenia techniczne pod osobny crawler API. Nie zmienia to niczego w istniejącym crawlerze chromedp — ten dokument jest wejściem do decyzji, nie jej zapisem.

---

## 1. Dlaczego w ogóle API

Obecny `NofluffjobsCollector` scrolluje wirtualizowany listing we własnym tabie z budżetem 3 minut, a `NofluffjobsParser` czyta pola regexpami z DOM-u. Portal wystawia jednak dwa publiczne endpointy JSON, które dają te same dane bez przeglądarki:

- listing: **6 requestów zamiast infinite-scrolla** dla najliczniejszej kategorii,
- oferta: pełne, ustrukturyzowane pola, w tym **data publikacji**, której obecny parser świadomie nie ustawia (`nofluffjobs_portal.go`, komentarz przy `PostedAt`).

Powód, dla którego data dziś nie istnieje, jest odtworzony i potwierdzony: payload `JobPosting` żyje w `<script type="application/ld+json">`, a Angular przy hydracji usuwa skrypty z DOM-u. Fixture `testdata/nofluffjobs/typical.html` (DOM po chromedp) ma **0** tagów `<script>`; `testdata/nofluffjobs/typical-server.html` (surowa odpowiedź serwera) ma ten blok. Dane nie zniknęły — zniknęły tylko z tego, co czyta obecny fetcher.

---

## 2. Listing — wszystkie oferty w kategorii

```
POST https://nofluffjobs.com/api/search/posting?salaryCurrency=PLN&salaryPeriod=month&region=pl&language=pl-PL&page={N}
Content-Type: application/json

{"rawSearch": "<term>"}
```

`salaryCurrency` i `salaryPeriod` są **wymagane**. Bez nich portal zwraca HTTP 400:
`{"status":400,"detail":"Required parameter 'salaryPeriod' is not present."}`

### 2.1 Mapowanie zatwierdzonych targetów na `rawSearch`

Siedem URL-i nofluffjobs z `approved-urls.md` przekłada się 1:1 na kryteria API. Każdy wiersz odpytany; kolumna „API rozwija do" to wartość `rawSearch` zwrócona przez serwer, kolumna `totalCount` to stan z dnia weryfikacji.

| Kategoria | Dotychczasowy URL | `rawSearch` w body | API rozwija do | `totalCount` |
|---|---|---|---|---|
| go | `/pl/go?sort=newest` | `go` | `requirement=Go` | 864 |
| go | `/pl/golang?sort=newest` | `golang` | `requirement=Golang` | 264 |
| php | `/pl/php?sort=newest` | `php` | `requirement=PHP` | 438 |
| architect | `/pl/architecture?sort=newest` | `architecture` | `category=architecture` | 790 |
| ai-ml | `/pl/artificial-intelligence?sort=newest` | `artificial-intelligence` | `category=artificial-intelligence` | 1109 |
| c | `/pl/c?sort=newest` | `c` | `requirement=C` | 529 |
| c | `/pl/c++?sort=newest` | `c++` | `requirement=C++` | 746 |

Akceptowany jest zarówno skrót (`"go"`), jak i forma kanoniczna (`"requirement=Go"`) — sprawdzone, dają identyczny wynik.

**Efekt uboczny:** znika problem z dosłownym `+` w ścieżce `/pl/c++`, który `approved-urls.md` oznaczał jako implementacyjną pułapkę. W API `c++` jedzie w body JSON, nie w URL-u, więc nie ma czego (i nie ma jak) źle zakodować.

### 2.2 Paginacja

Parametrem strony jest **`page`** (1-based). `limit`, `offset`, `pageSize` i `pageTo` są **przyjmowane, ale ignorowane** — zweryfikowane: `limit=20&offset=0`, `limit=20&offset=20`, `pageTo=1..3&pageSize=20` zwracają ten sam zestaw 167 pozycji. Dopiero `page=2` zwraca inny (136 pozycji, inna pierwsza oferta).

Odpowiedź niesie `totalPages` i `totalCount` — iteruj `page=1..totalPages`. Rozmiar strony jest zmienny i ustalany przez serwer (~130–330 pozycji), więc nie zakładaj stałej.

### 2.3 Dwie pułapki, które muszą trafić do implementacji

**Duplikaty między stronami.** Dla `go`: 6 stron → 864 pobrane pozycje, ale **808 unikalnych**. Oferty promowane (`topInSearch`, `highlighted`) pojawiają się na wielu stronach. Dedupe po polu `url` jest obowiązkowy, inaczej 56 ofert trafi do bazy dwa razy.

**Sortowanie.** `sort=newest` jest przyjmowany, ale kolejność nie jest ściśle malejąca po dacie — promowane wchodzą na górę niezależnie od wieku (zweryfikowane: przy `sort=newest` pierwsza oferta miała `posted` 2026-08-30, kolejne 2026-08-31). Skoro każda pozycja listingu niesie `posted` jako epoch ms, **kolejność ustalamy po stronie klienta**. To zamyka dla tego portalu otwartą kwestię „sortowanie: najnowsze pierwsze" z końcówki `approved-urls.md` — przestaje być zależnością od zachowania portalu.

### 2.4 Pola pozycji listingu (`postings[]`)

| Pole | Znaczenie |
|---|---|
| `url` | slug do endpointu szczegółów — **tego używać** |
| `id` | slug z diakrytykami (`...-michael-page-Wrocław`) — nie nadaje się jako klucz URL |
| `title` | tytuł oferty |
| `name` | nazwa firmy |
| `posted`, `renewed` | epoch ms |
| `location.places[].city`, `location.fullyRemote` | lokalizacja i flaga pełnej zdalności |
| `seniority`, `category`, `salary`, `logo` | metadane |
| `topInSearch`, `highlighted` | znaczniki promocji (przyczyna duplikatów) |

Poza tym w odpowiedzi: `totalCount`, `totalPages`, `criteriaSearch` (rozwinięte kryteria), `rawSearch`.

---

## 3. Szczegóły oferty

```
GET https://nofluffjobs.com/api/posting/{url}
```

Przykład zweryfikowany: `https://nofluffjobs.com/api/posting/lead-agentic-ai-consultant-softserve-remote` → HTTP 200, ~9 KB JSON.

Slug bierzemy wprost z `postings[].url`. Sprawdzone, że slug z listingu rozwiązuje się bez żadnej normalizacji (`mid-senior-kubernetes-engineer-michael-page-wroclaw` → 200).

### 3.1 Mapowanie na kolumny `possibilities`

| Kolumna | Pole API | Uwagi |
|---|---|---|
| `url` | `https://nofluffjobs.com/pl/job/` + `postingUrl` | `postingUrl` == `defaultUrl` == `id` |
| `job_title` | `title` | |
| `company_name` | `company.name` | |
| `tech` | `requirements.musts[].value` | dokładnie sekcja „Obowiązkowe", rozdzielona od `nices` |
| `description` | `details.description`, ewentualnie + `requirements.description` | HTML |
| `location` | `location.places[].city` | `"Remote"` dla w pełni zdalnych |
| `is_remote` | `location.remote == 5` (albo `places[0].city == "Remote"`) | patrz 3.2 |
| `posted_at` | `posted` (epoch ms) | **nowe** — dziś nieustawiane |
| `source` | stała `nofluffjobs.com` | |

### 3.2 `location.remote` to skala, nie boolean

Wartość jest liczbą 0–5 (dni zdalnych w tygodniu). Zmierzone: oferta stacjonarna w Katowicach → `2`, hybrydowa we Wrocławiu → `3`, w pełni zdalna → `5` i wtedy `places[0].city == "Remote"`. Traktowanie każdej niezerowej wartości jako „remote" zawyży wynik — obecny DOM-owy marker `data-cy="location_remote"` odpowiada dopiero `5`. Alternatywa bez dodatkowego requestu: `location.fullyRemote` z listingu.

### 3.3 Pozostałe dostępne pola

`requirements.nices[]`, `requirements.languages[]` (`{type:"MUST",code:"en"}`), `basics.category`, `basics.seniority[]`, `essentials.originalSalary` (`currency`, `types.b2b.range`, `period`, `disclosedAt`), `essentials.contract.start`, `specs.dailyTasks[]`, `benefits.benefits[]`, `benefits.equipment`, `benefits.officePerks[]`, `company.size` / `url` / `logo` / `video`, `expiresAt`, `status` (`PUBLISHED`), `renewed`, `postedOrRenewedDaysAgo`, `apply.option`, `recruitment.processSteps[]`, `seo.title` / `seo.description`, `regions[]`, `reference`.

---

## 4. JSON-LD jako wariant zapasowy

Gdyby endpointy API się zmieniły, ta sama strona oferty pobrana zwykłym GET-em (bez JS) zawiera jeden blok `<script type="application/ld+json">` z `@type: JobPosting`. Sprawdzone na 6 losowych ofertach z listingu `/pl/go?sort=newest` — **6/6** miało kompletny payload.

Pola JSON-LD zgadzają się z tym, co dziś wyciąga `NofluffjobsParser`:

| Pole | DOM (obecnie) | JSON-LD |
|---|---|---|
| tytuł | `<h1>` | `title` — identycznie |
| firma | `data-cy="JobOffer_CompanyProfile"` | `hiringOrganization.name` — identycznie |
| remote | `data-cy="location_remote"` → „100% współpraca zdalna" | `jobLocationType: "TELECOMMUTE"` |
| lokalizacja | brak dla stacjonarnych | `jobLocation.address.addressLocality` |
| data publikacji | brak | `datePosted` (`YYYY-MM-DD`) |

Dostępne dodatkowo: `description` (HTML), `employmentType`, `experienceRequirements`, `applicantLocationRequirements`, `jobBenefits`, `baseSalary`, `skills[]`, `industry`, `occupationalCategory`, `validThrough`.

**Ograniczenie, przez które JSON-LD jest wariantem zapasowym, a nie pierwszym wyborem:** `skills[]` to `musts + nices` w jednej liście, a atrybut `type: "main" | "other"` **nie** odpowiada temu podziałowi. Zweryfikowane na trzech ofertach: dla `mid-senior-kubernetes-engineer-michael-page-wroclaw` i `product-marketing-manager-xopero-software-warszawa` zachodzi `skills == musts + nices`, nigdy `skills == musts`; a w ofercie SoftServe „Deep learning" leży w sekcji „Obowiązkowe", ale w JSON-LD ma `type: "other"`. Odtworzenie obecnej semantyki `tech` = „tylko wymagane" jest więc z samego JSON-LD niemożliwe. `GET /api/posting/{url}` rozdziela `musts` i `nices` wprost.

---

## 5. Podsumowanie kosztu

| | Dziś (chromedp) | API |
|---|---|---|
| Listing | scroll wirtualizowanej listy w tabie, budżet 3 min/target | 4–7 requestów POST na target |
| Oferta | render strony + regexpy na DOM | 1 request GET, gotowy JSON |
| `posted_at` | niedostępne | `posted` |
| `tech` = tylko wymagane | tak (regex `branch="musts"`) | tak (`requirements.musts`) |
| Zależność od przeglądarki | tak | brak |

Dla 7 zatwierdzonych targetów: ~35 requestów listingu + 1 request na ofertę, bez chromedp.
