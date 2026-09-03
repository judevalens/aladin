package repo

// scanner is the common subset implemented by pgx rows and row results.
type scanner interface{ Scan(dest ...any) error }
