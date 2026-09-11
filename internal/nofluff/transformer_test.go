package nofluff

import (
	"encoding/json/v2"
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

func TestConvertOfferStoresRequirementsAsCommaSeparatedList(t *testing.T) {
	tests := []struct {
		name  string
		tiles string
		want  string
		valid bool
	}{
		{
			name: "joins requirements and leaves out the category",
			tiles: `{"values":[
				{"value":"business-analyst","type":"category"},
				{"value":"C++","type":"requirement"},
				{"value":"Java","type":"requirement"},
				{"value":"Python","type":"requirement"}
			]}`,
			want:  "C++, Java, Python",
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
