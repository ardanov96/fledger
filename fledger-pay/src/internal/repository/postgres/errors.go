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

// errAs is a tiny helper so call sites do not need to import errors.
func errAs(err error, target any) bool {
	type as interface{ As(any) bool }
	if a, ok := err.(as); ok {
		return a.As(target)
	}
	return false
}