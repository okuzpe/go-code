package tools

// Output and execution limits for MVP tools (see CLAUDE.md Tool Contract).
const (
	MaxReadFileBytes = 512 * 1024
	MaxReadFileLines = 400
	MaxBashOutput    = 256 * 1024
	BashTimeoutSec   = 30
	WebFetchTimeoutSec = 30
	maxWebFetchBytes   = 1 * 1024 * 1024
	MaxWebSearchResults = 8
	MaxSearchSnippet    = 2048
	WebSearchTimeoutSec = 15
	MaxRedirects        = 5
	MaxGlobMatches      = 500
	MaxGrepMatches      = 200
	MaxGrepFileBytes    = 512 * 1024
	MaxWriteFileBytes   = 1 * 1024 * 1024 // 1 MiB — write_file content cap and edit_file result cap
	// MaxPatchDiffBytes caps unified-diff input for the patch tool (parsed + applied in memory).
	MaxPatchDiffBytes = 1 * 1024 * 1024
)
