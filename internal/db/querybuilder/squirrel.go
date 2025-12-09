// Package querybuilder provides SQL query building utilities using Squirrel.
// It standardizes query construction across all repositories with SQLite-compatible
// placeholder formatting.
package querybuilder

import (
	sq "github.com/Masterminds/squirrel"
)

// QB is the pre-configured Squirrel statement builder for SQLite.
// It uses '?' placeholders which SQLite expects.
var QB = sq.StatementBuilder.PlaceholderFormat(sq.Question)

// Select creates a new SELECT query builder with the given columns.
func Select(columns ...string) sq.SelectBuilder {
	return QB.Select(columns...)
}

// Insert creates a new INSERT query builder for the given table.
func Insert(table string) sq.InsertBuilder {
	return QB.Insert(table)
}

// Update creates a new UPDATE query builder for the given table.
func Update(table string) sq.UpdateBuilder {
	return QB.Update(table)
}

// Delete creates a new DELETE query builder.
func Delete(table string) sq.DeleteBuilder {
	return QB.Delete(table)
}
