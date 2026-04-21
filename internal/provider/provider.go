package provider

import "iter"

// SessionSource identifies a session file or database to be parsed.
type SessionSource struct {
	Path     string
	Project  string
	Provider string
}

// ParsedCall is a single API call event from a provider parser.
type ParsedCall struct {
	Provider                 string
	Model                    string
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
	CachedInputTokens        int64
	ReasoningTokens          int64
	WebSearchRequests        int64
	CostUSD                  float64
	Tools                    []string
	Timestamp                string
	Speed                    string // "standard" or "fast"
	DeduplicationKey         string
	UserMessage              string
	SessionID                string
}

// Provider is the interface all data source adapters must implement.
type Provider interface {
	Name() string
	DiscoverSessions() ([]SessionSource, error)
	// ParseSession returns an iterator of parsed calls from a single source.
	// The iterator may yield errors; callers should handle them and continue.
	ParseSession(source SessionSource, seenKeys map[string]struct{}) iter.Seq2[ParsedCall, error]
}

// ContainsTool reports whether name is present in the tools slice.
func ContainsTool(tools []string, name string) bool {
	for _, t := range tools {
		if t == name {
			return true
		}
	}
	return false
}
