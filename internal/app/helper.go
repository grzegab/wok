package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// optText: pusty string -> NULL
func optText(s string) pgtype.Text {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

// optInt4: pusty string -> NULL, w innym wypadku parsuj
func optInt4(s string) (pgtype.Int4, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Int4{}, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return pgtype.Int4{}, err
	}
	return pgtype.Int4{Int32: int32(n), Valid: true}, nil
}

func formatDate(d pgtype.Date) string {
	if !d.Valid {
		return "-"
	}
	return d.Time.Format("2006-01-02")
}

func formatBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func formatPrice(p pgtype.Int4) string {
	if !p.Valid {
		return "-"
	}
	return fmt.Sprintf("%d", p.Int32)
}
