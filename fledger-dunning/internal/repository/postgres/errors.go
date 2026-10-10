package postgres

import "github.com/jackc/pgx/v5/pgconn"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errAs(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errAs(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	return false
}

func errAs(err error, target any) bool {
	type as interface{ As(any) bool }
	if a, ok := err.(as); ok {
		return a.As(target)
	}
	return false
}

// jsonMarshal / jsonUnmarshal are tiny indirection helpers.
func jsonMarshal(v any) ([]byte, error)    { return defaultJSONMarshal(v) }
func jsonUnmarshal(b []byte, v any) error { return defaultJSONUnmarshal(b, v) }