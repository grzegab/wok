package crawler

import "testing"

func TestOptDate(t *testing.T) {
	tests := []struct {
		name  string
		in    string
		valid bool
		want  string
	}{
		{"iso date", "2026-08-31", true, "2026-08-31"},
		{"padded", " 2026-01-05 ", true, "2026-01-05"},
		{"empty means unknown", "", false, ""},
		{"site format is not accepted", "31.08.2026", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := optDate(tt.in)

			if got.Valid != tt.valid {
				t.Fatalf("optDate(%q).Valid = %v, want %v", tt.in, got.Valid, tt.valid)
			}
			if tt.valid && got.Time.Format("2006-01-02") != tt.want {
				t.Fatalf("optDate(%q) = %s, want %s", tt.in, got.Time.Format("2006-01-02"), tt.want)
			}
		})
	}
}
