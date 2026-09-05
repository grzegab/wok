# Work Log

## 2026-09-04 — Implementation Started

**Total Steps**: 91
**Task Groups**: 14 (G1 weryfikacja produkcji, G2 schemat/DDL, G3 seed i dane, G4 sqlc, G5 rdzeń crawlera, G6 pipeline, G7-G10 cztery portale, G11 wiązanie main.go, G12 UI, G13 wdrożenie, G14 przegląd testów)

**Tryb wykonania**: SEKWENCYJNY.
Plan missing 'Files to Modify' on Group 1 — falling back to sequential execution.
Grupa 1 nie deklaruje ani `Dependencies:`, ani `Files to Modify:`. Jest bramką ręcznej weryfikacji produkcji i faktycznie nie tworzy plików, ale zgodnie z regułą walidacji executora brak pola nie jest interpretowany jako „None" — cichy błąd o rozłączności zbiorów plików jest dokładnie tym, przed czym ta reguła chroni.

**Bramka blokująca**: Grupa 1 wymaga dostępu do bazy produkcyjnej na VPS. Użytkownik został poproszony o ręczne wykonanie czterech zapytań (`current_schema()`, `SELECT url ... LIMIT 5`, licznik URL-i relatywnych, `DISTINCT category`). Do czasu podania wyników **Grupy 1, 2 i 3 pozostają wstrzymane** — bez nich nie da się rozstrzygnąć, czy powstaje `internal/database/data/001_normalize_urls.sql`, ani czy decyzja `category-key-mismatch` się broni.

**Kolejność startowa**: Grupa 5 (rdzeń crawlera) — jedyna grupa całkowicie niezablokowana, niezależna od bazy i od warstwy `sqlc`.

## Standards Reading Log

### Loaded Per Group
(wpisy dodawane w miarę wykonywania grup)

## 2026-09-04 — Grupa 5 zakończona (Rdzeń crawlera)

**Kroki**: 5.1 - 5.9 ukończone
**Wykonawca**: `maister:task-group-implementer` (Sonnet — przełączenie z Opusa po dwóch przerwaniach na limicie sesji; kroki 5.1-5.9 są rozpisane co do szczegółu, więc zakres swobody decyzyjnej był minimalny)

**Standardy zastosowane**
- Z planu: `global/minimal-implementation.md` (dokładnie dwa interfejsy, 1 i 2 metody, zero rejestrów i fabryk), `global/coding-style.md`, `global/error-handling.md` (`%w` przy `url.Parse` i `Evaluate`), `global/commenting.md` (komentarze „dlaczego", zero change-logów), `testing/test-writing.md` (tabela + `t.Run`, nazwy podtestów jako zdania angielskie, bez testify)
- Z INDEX.md: `global/conventions.md` — potwierdzono brak potrzeby zmiennych środowiskowych dla throttlingu
- Odkryte w trakcie: brak — kontrakty §9.1/§9.3/§9.6 były wystarczająco precyzyjne

**Testy**: `go test ./internal/crawler/ -run TestAbsoluteURL` → 6/6 PASS. Weryfikacja agenta głównego: `go build ./...`, `go vet ./...` czyste, pełne `go test ./internal/crawler/` → PASS (istniejące `TestOptDate`, `TestUnseen` bez regresji), `go list -deps ./cmd/web | grep chromedp` → zero trafień (kryterium 24).

**Pliki**
- `internal/crawler/fetcher.go` — usunięte `frequency`/`maxFrequencyJitter`/`listingTimeout`; dodane `LinkCollector`, `absoluteURL` (`:229`); `scrollAndCollectLinks`/`collectOfferLinks` z metod na funkcje pakietu sparametryzowane selektorem
- `internal/crawler/transformer.go` — usunięte `minOfferPause`/`maxOfferPause`; dodane `OfferParser`, `ParsedOffer`, `htmlReadyTimeout`, `fetchOfferHTML`
- `internal/crawler/throttling.go` (nowy), `internal/crawler/fetcher_test.go` (nowy), `internal/offer/placeholder.go` (nowy)

**Decyzje wykonawcy warte odnotowania**
1. **`listingTimeout` znika naprawdę, nie zmienia nazwy.** Zamiast nowej stałej pakietowej (co przeczyłoby rozstrzygnięciu M5 — „budżet czasu należy do implementacji `LinkCollector`"), `scrapeOnce` dostał wartość `3*time.Minute` inline z komentarzem. Zachowuje dotychczasowe zachowanie runtime bez tworzenia bytu, który Grupa 7 i tak zastąpi.
2. **Uniknięte cieniowanie nazw.** Pierwsza wersja miała pakietową stałą `offerCardSelector` obok parametru o tej samej nazwie — cieniowanie bez błędu kompilacji, ale mylące. Zastąpione literałem `".offer-card"` w jedynym wywołaniu.
3. **`fetchOfferHTML` nie jest jeszcze wpięty** w `transformBatch`/`scrapeOffer` — przepięcie pipeline'u należy do Grupy 6. Stan pośredni zgodny z oczekiwaniem; pakiet się kompiluje.
4. `htmlReadyTimeout = 15 s` — dolna granica przedziału 15-20 s ze spec, do korekty przy pierwszym prawdziwym portalu.

**Uwaga o diagnostyce**: LSP zgłosił `undefined: absoluteURL` i redeklarację `minOfferPause`/`maxOfferPause`. Zweryfikowane jako **nieaktualny cache** sprzed edycji — `go build`, `go vet` i `go test` przechodzą, `grep` nie pokazuje duplikatów.

## 2026-09-04 — Korekta trybu wykonania: powrót do fal równoległych

Grupa 1 nie deklarowała `Dependencies:` ani `Files to Modify:`, co zgodnie z regułą walidacji executora zdegradowało cały przebieg do trybu sekwencyjnego. To defekt planu, nie realna niewiadoma: Grupa 1 wykonuje wyłącznie zapytania `SELECT` na produkcji i nie zapisuje żadnego pliku. Pola uzupełnione w `implementation-plan.md` (`Files to Modify: None`), tryb falowy przywrócony.

**Następna fala**: Grupy 7-10 (cztery portale). Zależą wyłącznie od zamkniętej Grupy 5, a ich zbiory plików są parami rozłączne (osobny `<portal>_portal.go`, `<portal>_portal_test.go` i katalog `testdata/<portal>/`).

**Ograniczenie, które wymusza szeregowanie części pracy**: pierwszy krok każdej grupy portalowej to rozpoznanie DOM-u (kwestie otwarte 2 i 3), wymagające przeglądarki Playwright — a jest to **jedna współdzielona instancja**. Cztery agenty korzystające z niej równolegle zniszczyłyby sobie nawzajem sesje. Dlatego agent główny zbiera fixture'y HTML dla wszystkich czterech portali sam, szeregowo, zapisując je wprost do `internal/crawler/testdata/<portal>/`, a dopiero potem cztery implementacje parserów ruszają równolegle na zapisanych plikach, bez dostępu do przeglądarki.
