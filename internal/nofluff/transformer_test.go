package nofluff

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConvertOfferStoresOnlyCitiesAsLocation(t *testing.T) {
	tests := []struct {
		name     string
		location string
		want     string
		valid    bool
	}{
		{
			name: "keeps the city and drops the address and country",
			location: `{
				"places":[{
					"country":{"code":"POL","name":"Poland"},
					"city":"Wrocław",
					"street":"",
					"postalCode":"",
					"geoLocation":{"latitude":51.10929480000001,"longitude":17.0386019},
					"url":"analityk-automatyzacji-procesow-it-upvanta-wroclaw"
				}],
				"fullyRemote":false,
				"covidTimeRemotely":false,
				"hybridDesc":""
			}`,
			want:  "Wrocław",
			valid: true,
		},
		{
			name: "lists every city once and skips province-only places",
			location: `{"places":[
				{"city":"Remote"},
				{"city":"Kraków"},
				{"country":{"code":"POL","name":"Poland"},"province":"lower-silesian","provinceOnly":true},
				{"city":"Kraków"}
			]}`,
			want:  "Remote, Kraków",
			valid: true,
		},
		{
			name:     "an offer without places has no location",
			location: `{"places":[]}`,
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var offer RawOffer
			if err := json.Unmarshal([]byte(`{"location":`+tt.location+`}`), &offer); err != nil {
				t.Fatalf("could not decode offer: %v", err)
			}

			got, err := NewOfferTransformer(nil).ConvertOffer(offer)
			if err != nil {
				t.Fatalf("ConvertOffer() error = %v", err)
			}

			if got.Location.Valid != tt.valid || got.Location.String != tt.want {
				t.Errorf("ConvertOffer() location = %+v, want %q (valid %v)", got.Location, tt.want, tt.valid)
			}
		})
	}
}

func TestConvertOfferStoresNormalizedRequirementsAsCommaSeparatedList(t *testing.T) {
	tests := []struct {
		name  string
		tiles string
		want  string
		valid bool
	}{
		{
			name: "joins normalized requirements and leaves out the category",
			tiles: `{"values":[
				{"value":"business-analyst","type":"category"},
				{"value":"C++","type":"requirement"},
				{"value":"Java","type":"requirement"},
				{"value":"TypeScript","type":"requirement"}
			]}`,
			want:  "c++, java, typescript/javascript",
			valid: true,
		},
		{
			name:  "an offer with only a category has no tech",
			tiles: `{"values":[{"value":"data","type":"category"}]}`,
			valid: false,
		},
		{
			name:  "an offer without tiles has no tech",
			tiles: `{}`,
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var offer RawOffer
			if err := json.Unmarshal([]byte(`{"tiles":`+tt.tiles+`}`), &offer); err != nil {
				t.Fatalf("could not decode offer: %v", err)
			}

			got, err := NewOfferTransformer(nil).ConvertOffer(offer)
			if err != nil {
				t.Fatalf("ConvertOffer() error = %v", err)
			}

			if got.Tech.Valid != tt.valid || got.Tech.String != tt.want {
				t.Errorf("ConvertOffer() tech = %+v, want %q (valid %v)", got.Tech, tt.want, tt.valid)
			}
		})
	}
}

func TestConvertOfferStoresFullJobUrl(t *testing.T) {
	got, err := NewOfferTransformer(nil).ConvertOffer(RawOffer{Url: "go-developer-acme-warszawa"})
	if err != nil {
		t.Fatalf("ConvertOffer() error = %v", err)
	}

	want := "https://nofluffjobs.com/pl/job/go-developer-acme-warszawa"
	if got.Url != want {
		t.Errorf("ConvertOffer() url = %q, want %q", got.Url, want)
	}
}

func TestConvertOfferTakesRemoteFromLocation(t *testing.T) {
	var offer RawOffer
	if err := json.Unmarshal([]byte(`{"fullyRemote":false,"location":{"places":[],"fullyRemote":true}}`), &offer); err != nil {
		t.Fatalf("could not decode offer: %v", err)
	}

	got, err := NewOfferTransformer(nil).ConvertOffer(offer)
	if err != nil {
		t.Fatalf("ConvertOffer() error = %v", err)
	}

	if !got.IsRemote {
		t.Error("ConvertOffer() is_remote = false, want true")
	}
}

func TestFillOfferFailsOnNonOkResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer server.Close()

	if _, err := NewOfferTransformer(nil).FillOffer(context.Background(), RawOffer{}, server.URL); err == nil {
		t.Fatal("FillOffer() should fail when the posting is not found")
	}
}

func TestFillOfferStopsWhenContextIsCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := NewOfferTransformer(nil).FillOffer(ctx, RawOffer{}, server.URL); err == nil {
		t.Fatal("FillOffer() should fail with a cancelled context")
	}
}
