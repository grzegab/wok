package nofluff

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/grzegab/wok/internal/category"
)

// listingHandler serves a fake nofluff listing: pages[i] is the offer count of
// page i+1, and any page past the slice comes back empty.
func listingHandler(t *testing.T, pages []int, totalPages int, requested *[]string) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		*requested = append(*requested, page)

		n, err := strconv.Atoi(page)
		if err != nil {
			t.Errorf("listing asked for page %q, want a number", page)
		}

		offers := 0
		if n >= 1 && n <= len(pages) {
			offers = pages[n-1]
		}

		postings := make([]string, 0, offers)
		for i := range offers {
			postings = append(postings, fmt.Sprintf(`{"url":"offer-%s-%d","name":"Acme","title":"Go dev"}`, page, i))
		}

		fmt.Fprintf(w, `{"postings":[%s],"totalCount":99,"totalPages":%d}`, strings.Join(postings, ","), totalPages)
	}
}

func TestFetchAll(t *testing.T) {
	tests := []struct {
		name       string
		pages      []int
		totalPages int
		wantOffers int
		wantPages  []string
	}{
		{
			name:       "walks every page the listing reports",
			pages:      []int{3, 3, 2},
			totalPages: 3,
			wantOffers: 8,
			wantPages:  []string{"1", "2", "3"},
		},
		{
			name:       "stops at the first empty page when no count is given",
			pages:      []int{3, 1},
			wantOffers: 4,
			wantPages:  []string{"1", "2", "3"},
		},
		{
			name:       "a single page listing is fetched once",
			pages:      []int{2},
			totalPages: 1,
			wantOffers: 2,
			wantPages:  []string{"1"},
		},
		{
			name:       "an empty listing yields nothing",
			pages:      nil,
			totalPages: 0,
			wantOffers: 0,
			wantPages:  []string{"1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requested []string
			srv := httptest.NewServer(listingHandler(t, tt.pages, tt.totalPages, &requested))
			defer srv.Close()

			of := NewNoFluffFetcher([]category.NoFluffCategory{
				{Category: category.Category{Slug: "go", Name: "Go", ScrapeURL: srv.URL}, SearchTerm: "go"},
			})

			of.fetchAll(t.Context())
			close(of.RawOfferBuff)

			offers := 0
			for o := range of.RawOfferBuff {
				if o.Category != "Go" {
					t.Errorf("offer category = %q, want %q", o.Category, "Go")
				}
				offers++
			}

			if offers != tt.wantOffers {
				t.Errorf("fetchAll() collected %d offers, want %d", offers, tt.wantOffers)
			}

			if !slices.Equal(requested, tt.wantPages) {
				t.Errorf("fetchAll() requested pages %v, want %v", requested, tt.wantPages)
			}
		})
	}
}

func TestFetchAllStopsWhenContextIsCancelled(t *testing.T) {
	var requested []string
	srv := httptest.NewServer(listingHandler(t, []int{5, 5}, 2, &requested))
	defer srv.Close()

	of := NewNoFluffFetcher([]category.NoFluffCategory{
		{Category: category.Category{Slug: "go", Name: "Go", ScrapeURL: srv.URL}, SearchTerm: "go"},
	})
	of.RawOfferBuff = make(chan RawOffer, 1)

	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(50*time.Millisecond, cancel)

	of.fetchAll(ctx)

	if len(requested) != 1 {
		t.Errorf("fetchAll() requested pages %v, want to stop after the first", requested)
	}
}

// TestFetchAllReadsRealListing guards the decoding against the shape nofluff
// actually answers with, page count included.
func TestFetchAllReadsRealListing(t *testing.T) {
	listing, err := os.ReadFile(filepath.Join("testdata", "reponse_1.json"))
	if err != nil {
		t.Fatalf("could not read the recorded listing: %v", err)
	}

	var requested []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Query().Get("page"))
		w.Write(listing)
	}))
	defer srv.Close()

	of := NewNoFluffFetcher([]category.NoFluffCategory{
		{Category: category.Category{Slug: "go", Name: "Go", ScrapeURL: srv.URL}, SearchTerm: "go"},
	})

	offers := make(chan int)
	go func() {
		count := 0
		for range of.RawOfferBuff {
			count++
		}
		offers <- count
	}()

	of.fetchAll(t.Context())
	close(of.RawOfferBuff)

	// The recorded listing reports 6 pages and carries 191 offers on each.
	if want := []string{"1", "2", "3", "4", "5", "6"}; !slices.Equal(requested, want) {
		t.Errorf("fetchAll() requested pages %v, want %v", requested, want)
	}

	if got, want := <-offers, 6*191; got != want {
		t.Errorf("fetchAll() collected %d offers, want %d", got, want)
	}
}

func TestPause(t *testing.T) {
	tests := []struct {
		name       string
		wait       time.Duration
		arrange    func(cancel context.CancelFunc)
		want       bool
		maxElapsed time.Duration
	}{
		{
			name:       "waits out the delay and reports it finished",
			wait:       time.Millisecond,
			want:       true,
			maxElapsed: time.Second,
		},
		{
			name:       "an already cancelled context returns at once",
			wait:       time.Hour,
			arrange:    func(cancel context.CancelFunc) { cancel() },
			want:       false,
			maxElapsed: 100 * time.Millisecond,
		},
		{
			name:       "cancelling mid-wait cuts the delay short",
			wait:       time.Hour,
			arrange:    func(cancel context.CancelFunc) { time.AfterFunc(10*time.Millisecond, cancel) },
			want:       false,
			maxElapsed: time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tt.arrange != nil {
				tt.arrange(cancel)
			}

			start := time.Now()
			got := pause(ctx, tt.wait)
			elapsed := time.Since(start)

			if got != tt.want {
				t.Errorf("pause() got = %v, want %v", got, tt.want)
			}

			if elapsed > tt.maxElapsed {
				t.Errorf("pause() elapsed = %v, want %v", elapsed, tt.maxElapsed)
			}
		})
	}
}
