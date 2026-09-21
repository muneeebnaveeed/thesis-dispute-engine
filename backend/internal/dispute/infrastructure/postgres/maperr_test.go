package postgres

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/muneeebnaveeed/thesis-dispute-engine/backend/internal/dispute/application"
)

func TestMapErr(t *testing.T) {
	cases := map[string]struct {
		in   error
		want error
	}{
		"no rows":              {pgx.ErrNoRows, application.ErrNotFound},
		"unique violation":     {&pgconn.PgError{Code: "23505"}, application.ErrConflict},
		"serialization":        {&pgconn.PgError{Code: "40001"}, application.ErrConflict},
		"deadlock":             {&pgconn.PgError{Code: "40P01"}, application.ErrConflict},
		"connection exception": {&pgconn.PgError{Code: "08006"}, application.ErrUnavailable},
		"too many connections": {&pgconn.PgError{Code: "53300"}, application.ErrUnavailable},
		"admin shutdown":       {&pgconn.PgError{Code: "57P01"}, application.ErrUnavailable},
		"deadline":             {context.DeadlineExceeded, application.ErrUnavailable},
		"dial failure":         {&net.OpError{Op: "dial", Err: errors.New("refused")}, application.ErrUnavailable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := mapErr(tc.in); !errors.Is(got, tc.want) {
				t.Fatalf("mapErr(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
	other := errors.New("constraint check")
	if got := mapErr(&pgconn.PgError{Code: "23514"}); errors.Is(got, application.ErrUnavailable) || errors.Is(got, application.ErrConflict) {
		t.Fatalf("check violation should pass through, got %v", got)
	}
	if got := mapErr(other); got != other { //nolint:errorlint // identity is the point
		t.Fatalf("unclassified error should pass through unchanged")
	}
}
