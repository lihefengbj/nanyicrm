package migrations

import "embed"

// FS contains the immutable production schema migrations shipped with the
// backend binary.
//
//go:embed *.up.sql *.down.sql
var FS embed.FS
