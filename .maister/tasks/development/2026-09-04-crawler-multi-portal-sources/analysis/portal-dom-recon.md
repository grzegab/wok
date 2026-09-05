# Rozpoznanie DOM czterech portali

Data: 2026-09-04
Metoda: realna przeglądarka (Playwright), listing + strona oferty każdego portalu.
Zamyka **kwestie otwarte 2 i 3** ze specyfikacji (mechanika listingu i selektory gotowości).

Fixture'y: `internal/crawler/testdata/<portal>/typical.html` — surowy `outerHTML` po wyrenderowaniu, z usuniętymi `<script>`, `<style>` i `<link>` (parsery czytają DOM, nie payloady frameworka; usunięcie zmniejszyło pliki 3-8×).

---

## Zestawienie zbiorcze

| Portal | Selektor karty na listingu | Postać href | Mechanika listingu | Tytuł | Firma | Data publikacji |
|---|---|---|---|---|---|---|
| justjoin.it | `.offer-card` | **absolutny** | infinite scroll (wirtualizowany) | `h1` | `h2` | `Published: DD.MM.YYYY` w treści |
| nofluffjobs.com | `a.posting-list-item` | **relatywny** (`/pl/job/...`) | brak paginacji URL-owej | `h1` | atrybuty `data-cy` | **względna** („6 dni") |
| bulldogjob.pl | `a[href*="/companies/jobs/"]` + wzorzec `\d+-` | **absolutny** | **brak paginacji i brak doładowywania — twardy limit 50 ofert** | `h1` | `h2` | **brak** |
| solid.jobs | `a[href*="/offer/"]` | **relatywny** (`/offer/<id>/<slug>`) | do ustalenia | `h1` | `<title>` / `og:title` | **brak** |

---

## justjoin.it

- Karty: `.offer-card`, href **już absolutny** (`https://justjoin.it/job-offer/...`) — `absoluteURL` jest tu no-opem, ale musi to obsłużyć bez zmiany wartości (przypadek „already-absolute href is returned unchanged" w `TestAbsoluteURL`).
- Oferta: `h1` = tytuł, `h2` = firma, `h3` „Job description", `h3` „Tech stack" z następującymi `h4` jako technologie, `Published: DD.MM.YYYY` w tekście.
- Fixture: 152 KB. Zachowuje wszystkie powyższe markery — zweryfikowane.
- Selektor gotowości (`ReadySelector`): `h1`.

## nofluffjobs.com

- Karty: `a.posting-list-item` (klasa stabilna, semantyczna), rodzic `.list-container`.
- **href relatywny** — to jest dokładnie ten portal, dla którego `absoluteURL` jest niezbędne.
- Sortowanie `?sort=newest` zweryfikowane wcześniej (zmienia kolejność wyników).
- Oferta: `h1` = tytuł. **Firma nie jest w `h2`** — `h2` to nagłówki sekcji („Obowiązkowe", „Opis wymagań", „Zakres obowiązków"). Stabilne haki to atrybuty `data-cy` (157 wystąpień w fixture), m.in. `JobOffer_CompanyProfile`, `title position on the job offer listing`, `location on the job offer listing`, `location_remote`.
- **Data publikacji jest względna** („6 dni"). Data `10.09.2026` obecna w dokumencie to termin **ważności** oferty (przyszłość względem dnia rozpoznania), nie publikacja. Parser musi albo przeliczyć względną datę na absolutną, albo zostawić `posted_at` puste — nie wolno pomylić jej z datą ważności.
- Fixture: 160 KB.
- Selektor gotowości: `h1` albo `[data-cy="JobOffer_CompanyProfile"]`.

## bulldogjob.pl

- Karty: klasa `JobListItem_item__fYh8y` to **zahaszowany CSS-module** — niestabilna między buildami, **nie używać jako selektora**. Filtrować po wzorcu URL-a: `a[href*="/companies/jobs/"]` + regex `\/companies\/jobs\/\d+-`.
- href **absolutny**.
- **Brak paginacji.** Zweryfikowane trzy warianty URL-a (`?page=2`, `/2`, `?p=2`) — wszystkie zwracają tę samą pierwszą ofertę i ten sam zbiór 50 pozycji. Brak przycisku „więcej"/„load more". Dwanaście przewinięć strony nie doładowało ani jednej oferty (50 → 50).
- **Konsekwencja dla planu:** Grupa 9 **nie potrzebuje logiki paginacji**, a `maxListingPages = 3` jest dla tego portalu bezprzedmiotowe. Jednocześnie 50 ofert to twardy sufit na filtr — głębszej historii z Bulldogjoba nie da się sięgnąć żadnym URL-em.
- Oferta: `h1` = tytuł, `h2` = firma (występuje dwa razy, ta sama wartość), `h3` „What we offer" / „What we expect".
- **Brak jakiejkolwiek daty publikacji** — `posted_at` zostaje puste.
- Fixture: 44 KB (najlżejsza strona z czterech).
- Selektor gotowości: `h1`.

## solid.jobs

- Karty: `a[href*="/offer/"]`, klasa `card ... offer-compact` (semantyczna, ale ogólna).
- **href relatywny** (`/offer/37830/appliscale-backend-engineer-golang`).
- Portal przekierowuje na adres kanoniczny — **zapisywać URL końcowy po przekierowaniu**, nie ten z listingu (to jedna z „trzech rzeczy, które zabijają zadanie po cichu" z notatek planu).
- Oferta: `h1` = tytuł. **Firma nie jest w `h2`** — `h2` to numerowane nagłówki sekcji („01 Zakres zadań", „02 Wymagania", „03 Profil", „04 Oferujemy"). Firma jest niezawodnie w `<title>` oraz `og:title` we wzorcu `<Tytuł> @<Firma> – SOLID.Jobs`.
- **Brak daty publikacji** — `posted_at` zostaje puste.
- Fixture: 92 KB.
- Selektor gotowości: `h1`.

---

## Wnioski dla implementacji

1. **`absoluteURL` jest realnie potrzebne dla dwóch z czterech portali** (nofluffjobs, solid.jobs). Dla justjoin.it i bulldogjob musi być przezroczyste.
2. **Trzy z czterech portali nie podają daty publikacji w postaci absolutnej.** `posted_at` będzie puste dla bulldogjob i solid.jobs, a dla nofluffjobs wymaga przeliczenia daty względnej. Tylko justjoin.it daje datę wprost. To istotnie ogranicza użyteczność sortowania po `posted_at`.
3. **Firma wymaga innego haka na każdym portalu.** `h2` działa tylko dla justjoin.it i bulldogjob. Założenie „h2 = firma" z obecnego kodu **nie generalizuje się**.
4. **Żaden portal poza justjoin.it nie ma infinite scrolla wymagającego wirtualizacji.** `scrollAndCollectLinks` zostaje specyficzne dla justjoin.it; bulldogjob renderuje komplet od razu.
5. **Fixture'y edge-case** (`no-date`, `no-stack`, `remote`, `unparsable`) należy wyprowadzić z zapisanego `typical.html` przez usunięcie odpowiednich fragmentów DOM — to standardowa i uczciwa praktyka, bo znalezienie w naturze oferty z każdą kombinacją braków jest nierealne. `unparsable.html` ma nie zawierać `h1`.

---

# KOREKTA 2026-09-04: wszystkie cztery portale publikują JSON-LD `JobPosting`

**Przyczyna korekty — błąd agenta głównego.** Pierwsza tura fixture'ów została odchudzona przez usunięcie **wszystkich** znaczników `<script>`. Wycięło to `<script type="application/ld+json">`, czyli ustrukturyzowany `JobPosting` ze schema.org. Grupy 7-10 zbudowały parsery na regeksach po DOM-ie i na podstawie okrojonych plików wyciągnęły wnioski, które są **nieprawdziwe**:

- „Bulldogjob nie ma daty publikacji" → **ma** (`datePosted: 2026-09-04`)
- „Bulldogjob nie ma stacku technologicznego" → **ma** (`skills: "TypeScript, Cursor, Claude Code"`)
- „solid.jobs nie ma daty publikacji" → **ma** (`datePosted: 2026-09-04`)
- „NoFluffJobs ma tylko datę ważności, nie publikacji" → **ma `datePosted: 2026-08-11`**, tylko nie w wyrenderowanym DOM-ie

Fixture'y `typical.html` zostały wygenerowane ponownie z zachowaniem bloków `ld+json`. Pliki pochodne (`no-date`, `no-stack`, `remote`, `unparsable`) trzeba wyprowadzić na nowo.

## Co zawiera `JobPosting` na każdym portalu

| Portal | `datePosted` | Tytuł | Firma | `jobLocationType` | `skills` | Blok w wyrenderowanym DOM? |
|---|---|---|---|---|---|---|
| justjoin.it | `2026-09-02T11:48:02.408111Z` (pełny timestamp ISO) | tak | `Andersen` | `TELECOMMUTE` | brak | **tak** |
| bulldogjob.pl | `2026-09-04` | tak | `Devire` | `TELECOMMUTE` | `"TypeScript, Cursor, Claude Code"` (string po przecinku) | **tak** |
| solid.jobs | `2026-09-04` | tak | `Appliscale` | `TELECOMMUTE` | `"Golang, CI/CD, TypeScript, Docker, ..."` (string po przecinku) | **tak** |
| nofluffjobs.com | `2026-08-11` | tak | `Cast ai` | `TELECOMMUTE` | `[{"value":"Go","type":"main"}, ...]` (**tablica obiektów**) | **NIE** |

Dodatkowo justjoin.it ma `jobLocation.address.addressLocality` (`Praha`) i `addressCountry` (`CZ`) — lokalizacja strukturalnie, bez heurystyki „linia po tytule".

## Kształty do obsłużenia

1. **Opakowanie**: solid.jobs, bulldogjob i justjoin mają `JobPosting` jako obiekt najwyższego poziomu albo element tablicy. NoFluffJobs pakuje go w `{"@context":..., "@graph":[ Organization, BreadcrumbList, JobPosting ]}`. Parser musi obsłużyć: obiekt, tablicę oraz `@graph`.
2. **`skills`**: string rozdzielony przecinkami (bulldogjob, solid.jobs) **albo** tablica obiektów `{value, type}` (NoFluffJobs). Docelowy format kolumny `tech` to string rozdzielony `", "`.
3. **`datePosted`**: sama data `YYYY-MM-DD` (bulldogjob, solid.jobs, NoFluffJobs) albo pełny timestamp RFC 3339 (justjoin.it). Obcinać do daty.
4. **`jobLocationType: "TELECOMMUTE"`** to strukturalny sygnał pracy zdalnej na wszystkich czterech portalach — zastępuje kruche heurystyki tekstowe (`strings.Contains(body,"Remote")`, ikony `lucide-map-pinned`, znaczniki `sj-green-or-red-mark`), które grupy 7-10 musiały wymyślać.

## NoFluffJobs — blok znika po hydracji

Zweryfikowane: `document.querySelectorAll('script[type="application/ld+json"]')` na wyrenderowanej stronie zwraca **0** elementów, natomiast surowy HTML z serwera zawiera **1** blok z kompletnym `JobPosting`. Angular usuwa go przy bootstrapie.

**Decyzja użytkownika (2026-09-04):** dla NoFluffJobs crawler wykonuje dodatkowo jedno lekkie `http.Get` po surowy HTML serwera i z niego wyciąga `@graph` → `JobPosting`. Podwaja liczbę żądań do tego jednego portalu, ale przy pauzie 2-3 min na ofertę tempo pozostaje bardzo zachowawcze, a żądanie jest tanie (bez renderowania).

Fixture dla tej ścieżki: `internal/crawler/testdata/nofluffjobs/typical-server.html` (surowy HTML serwera, 149 KB, zawiera blok `ld+json`). Fixture `typical.html` pozostaje **wiernym** zrzutem wyrenderowanego DOM-u — celowo **bez** `ld+json`, bo tak wygląda rzeczywistość widziana przez chromedp.

## Wynikający kontrakt parsera

**JSON-LD jako źródło pierwszego wyboru, DOM jako zapasowe.** Parser najpierw próbuje `JobPosting`; pola, których tam nie ma (np. opis stanowiska, `skills` dla justjoin.it), dobiera dotychczasową ekstrakcją po DOM. Gdy bloku nie ma w ogóle — pełne zejście na ścieżkę DOM, która jest już zaimplementowana i przetestowana w grupach 7-10.

Zysk: `posted_at` wypełnione dla wszystkich czterech portali zamiast jednego, `tech` również dla Bulldogjoba, a tytuł, firma i tryb pracy przestają zależeć od układu nagłówków, który zmienia się przy każdym redesignie.
