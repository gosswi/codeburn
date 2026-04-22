package types

// TaskCategory represents the classification of a user interaction.
type TaskCategory string

const (
	CategoryCoding       TaskCategory = "coding"
	CategoryDebugging    TaskCategory = "debugging"
	CategoryFeature      TaskCategory = "feature"
	CategoryRefactoring  TaskCategory = "refactoring"
	CategoryTesting      TaskCategory = "testing"
	CategoryExploration  TaskCategory = "exploration"
	CategoryPlanning     TaskCategory = "planning"
	CategoryDelegation   TaskCategory = "delegation"
	CategoryGit          TaskCategory = "git"
	CategoryBuildDeploy  TaskCategory = "build/deploy"
	CategoryConversation TaskCategory = "conversation"
	CategoryBrainstorm   TaskCategory = "brainstorming"
	CategoryGeneral      TaskCategory = "general"
)

var CategoryLabels = map[TaskCategory]string{
	CategoryCoding:       "Coding",
	CategoryDebugging:    "Debugging",
	CategoryFeature:      "Feature Dev",
	CategoryRefactoring:  "Refactoring",
	CategoryTesting:      "Testing",
	CategoryExploration:  "Exploration",
	CategoryPlanning:     "Planning",
	CategoryDelegation:   "Delegation",
	CategoryGit:          "Git Ops",
	CategoryBuildDeploy:  "Build/Deploy",
	CategoryConversation: "Conversation",
	CategoryBrainstorm:   "Brainstorming",
	CategoryGeneral:      "General",
}

type TokenUsage struct {
	InputTokens             int64
	OutputTokens            int64
	CacheCreationInputTokens int64
	CacheReadInputTokens    int64
	CachedInputTokens       int64
	ReasoningTokens         int64
	WebSearchRequests       int64
}

type ParsedApiCall struct {
	Provider        string
	Model           string
	Usage           TokenUsage
	CostUSD         float64
	Tools           []string
	McpTools        []string
	HasAgentSpawn   bool
	HasPlanMode     bool
	Speed           string // "standard" or "fast"
	Timestamp       string
	BashCommands    []string
	DeduplicationKey string
}

type ParsedTurn struct {
	UserMessage    string
	AssistantCalls []ParsedApiCall
	Timestamp      string
	SessionID      string
}

type ClassifiedTurn struct {
	ParsedTurn
	Category  TaskCategory
	Retries   int
	HasEdits  bool
}

type CategoryStats struct {
	Turns       int
	CostUSD     float64
	Retries     int
	EditTurns   int
	OneShotTurns int
}

type ModelStats struct {
	Calls   int
	CostUSD float64
	Tokens  TokenUsage
}

type SessionSummary struct {
	SessionID               string
	Project                 string
	FirstTimestamp          string
	LastTimestamp           string
	TotalCostUSD            float64
	TotalInputTokens        int64
	TotalOutputTokens       int64
	TotalCacheReadTokens    int64
	TotalCacheWriteTokens   int64
	APICalls                int
	Turns                   []ClassifiedTurn
	ModelBreakdown          map[string]ModelStats
	ToolBreakdown           map[string]struct{ Calls int }
	McpBreakdown            map[string]struct{ Calls int }
	BashBreakdown           map[string]struct{ Calls int }
	CategoryBreakdown       map[TaskCategory]CategoryStats
}

type ProjectSummary struct {
	Project      string
	ProjectPath  string
	Sessions     []SessionSummary
	TotalCostUSD  float64
	TotalAPICalls int
}

type DateRange struct {
	Start int64 // Unix milliseconds
	End   int64 // Unix milliseconds
}

type ParseOptions struct {
	DateRange      *DateRange
	ProviderFilter string
	ExtractBash    bool
}
