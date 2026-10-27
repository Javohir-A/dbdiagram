package config

import "time"

const (
	AppName = "dbdiagram"

	// Paging bounds. A request asking for more gets MaxLimit, not a table scan.
	DefaultLimit int32 = 10
	MaxLimit     int32 = 1000

	// Field bounds. The body limit sits above MaxDBMLSize so the service, not
	// the transport, reports an oversized DBML with a useful message.
	MaxNameLength   = 120
	MaxDBMLSize     = 1 << 20 // 1 MiB
	MaxBodySize     = 2 << 20 // 2 MiB
	MaxPositions    = 5000
	MaxCoordinate   = 1e7
	ShareTokenBytes = 16

	// History: a save that changes the DBML snapshots the state before it,
	// but only when the newest snapshot is at least VersionInterval old, so
	// autosave does not write a row per keystroke burst.
	VersionInterval = 5 * time.Minute
	MaxVersions     = 100
	MaxLabelLength  = 120

	// Accounts and folders.
	MinPasswordLength = 8
	MaxPasswordBytes  = 72 // bcrypt silently ignores anything past this
	MaxEmailLength    = 254
	MaxFolderDepth    = 8

	// Personal access tokens.
	MaxTokenNameLength = 60
	DefaultTokenDays   = 90
	MaxTokenDays       = 365
	MaxActiveTokens    = 20

	// ManualVersionLabel names a manual snapshot saved without a label, so it
	// can be told apart from an automatic one (whose label is empty).
	ManualVersionLabel = "Manual save"
)
