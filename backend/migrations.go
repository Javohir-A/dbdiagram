// Package migrations embeds the SQL files so the binary carries the whole
// schema.
package migrations

import "embed"

// FS holds migrations/<set>/NNNNNN_<name>.{up,down}.sql.
//
//go:embed migrations
var FS embed.FS
