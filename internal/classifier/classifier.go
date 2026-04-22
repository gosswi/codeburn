package classifier

import (
	"regexp"
	"strings"

	"github.com/agentseal/codeburn/internal/types"
)

var (
	testPatterns    = regexp.MustCompile(`(?i)\b(test|pytest|vitest|jest|mocha|spec|coverage|npm\s+test|npx\s+vitest|npx\s+jest)\b`)
	gitPatterns     = regexp.MustCompile(`(?i)\bgit\s+(push|pull|commit|merge|rebase|checkout|branch|stash|log|diff|status|add|reset|cherry-pick|tag)\b`)
	buildPatterns   = regexp.MustCompile(`(?i)\b(npm\s+run\s+build|npm\s+publish|pip\s+install|docker|deploy|make\s+build|npm\s+run\s+dev|npm\s+start|pm2|systemctl|brew|cargo\s+build)\b`)
	installPatterns = regexp.MustCompile(`(?i)\b(npm\s+install|pip\s+install|brew\s+install|apt\s+install|cargo\s+add)\b`)

	debugKeywords     = regexp.MustCompile(`(?i)\b(fix|bug|error|broken|failing|crash|issue|debug|traceback|exception|stack\s*trace|not\s+working|wrong|unexpected|status\s+code|404|500|401|403)\b`)
	featureKeywords   = regexp.MustCompile(`(?i)\b(add|create|implement|new|build|feature|introduce|set\s*up|scaffold|generate|make\s+(?:a|me|the)|write\s+(?:a|me|the))\b`)
	refactorKeywords  = regexp.MustCompile(`(?i)\b(refactor|clean\s*up|rename|reorganize|simplify|extract|restructure|move|migrate|split)\b`)
	brainstormKeywords = regexp.MustCompile(`(?i)\b(brainstorm|idea|what\s+if|explore|think\s+about|approach|strategy|design|consider|how\s+should|what\s+would|opinion|suggest|recommend)\b`)
	researchKeywords  = regexp.MustCompile(`(?i)\b(research|investigate|look\s+into|find\s+out|check|search|analyze|review|understand|explain|how\s+does|what\s+is|show\s+me|list|compare)\b`)

	filePatterns   = regexp.MustCompile(`(?i)\.(py|js|ts|tsx|jsx|json|yaml|yml|toml|sql|sh|go|rs|java|rb|php|css|html|md|csv|xml)\b`)
	scriptPatterns = regexp.MustCompile(`(?i)\b(run\s+\S+\.\w+|execute|scrip?t|curl|api\s+\S+|endpoint|request\s+url|fetch\s+\S+|query|database|db\s+\S+)\b`)
	urlPattern     = regexp.MustCompile(`(?i)https?://\S+`)
)

var editTools = map[string]struct{}{
	"Edit":         {},
	"Write":        {},
	"FileEditTool": {},
	"FileWriteTool": {},
	"NotebookEdit": {},
	"cursor:edit":  {},
}

var readTools = map[string]struct{}{
	"Read":        {},
	"Grep":        {},
	"Glob":        {},
	"FileReadTool": {},
	"GrepTool":    {},
	"GlobTool":    {},
}

// BashTools is exported for use by the parser to identify bash commands.
var BashTools = map[string]struct{}{
	"Bash":           {},
	"BashTool":       {},
	"PowerShellTool": {},
}

var taskTools = map[string]struct{}{
	"TaskCreate": {},
	"TaskUpdate": {},
	"TaskGet":    {},
	"TaskList":   {},
	"TaskOutput": {},
	"TaskStop":   {},
	"TodoWrite":  {},
}

var searchTools = map[string]struct{}{
	"WebSearch": {},
	"WebFetch":  {},
	"ToolSearch": {},
}

func inSet(tool string, set map[string]struct{}) bool {
	_, ok := set[tool]
	return ok
}

func getAllTools(turn types.ParsedTurn) []string {
	var tools []string
	for _, call := range turn.AssistantCalls {
		tools = append(tools, call.Tools...)
	}
	return tools
}

func hasEditTools(tools []string) bool {
	for _, t := range tools {
		if inSet(t, editTools) {
			return true
		}
	}
	return false
}

func hasReadTools(tools []string) bool {
	for _, t := range tools {
		if inSet(t, readTools) {
			return true
		}
	}
	return false
}

func hasBashTool(tools []string) bool {
	for _, t := range tools {
		if inSet(t, BashTools) {
			return true
		}
	}
	return false
}

func hasTaskTools(tools []string) bool {
	for _, t := range tools {
		if inSet(t, taskTools) {
			return true
		}
	}
	return false
}

func hasSearchTools(tools []string) bool {
	for _, t := range tools {
		if inSet(t, searchTools) {
			return true
		}
	}
	return false
}

func hasMcpTools(tools []string) bool {
	for _, t := range tools {
		if strings.HasPrefix(t, "mcp__") {
			return true
		}
	}
	return false
}

func hasSkillTool(tools []string) bool {
	for _, t := range tools {
		if t == "Skill" {
			return true
		}
	}
	return false
}

func classifyByToolPattern(turn types.ParsedTurn) (types.TaskCategory, bool) {
	tools := getAllTools(turn)
	if len(tools) == 0 {
		return "", false
	}

	for _, call := range turn.AssistantCalls {
		if call.HasPlanMode {
			return types.CategoryPlanning, true
		}
	}
	for _, call := range turn.AssistantCalls {
		if call.HasAgentSpawn {
			return types.CategoryDelegation, true
		}
	}

	hasEdits := hasEditTools(tools)
	hasReads := hasReadTools(tools)
	hasBash := hasBashTool(tools)
	hasTasks := hasTaskTools(tools)
	hasSearch := hasSearchTools(tools)
	hasMcp := hasMcpTools(tools)
	hasSkill := hasSkillTool(tools)

	if hasBash && !hasEdits {
		userMsg := turn.UserMessage
		if testPatterns.MatchString(userMsg) {
			return types.CategoryTesting, true
		}
		if gitPatterns.MatchString(userMsg) {
			return types.CategoryGit, true
		}
		if buildPatterns.MatchString(userMsg) {
			return types.CategoryBuildDeploy, true
		}
		if installPatterns.MatchString(userMsg) {
			return types.CategoryBuildDeploy, true
		}
	}

	if hasEdits {
		return types.CategoryCoding, true
	}

	if hasBash && hasReads {
		return types.CategoryExploration, true
	}
	if hasBash {
		return types.CategoryCoding, true
	}

	if hasSearch || hasMcp {
		return types.CategoryExploration, true
	}
	if hasReads && !hasEdits {
		return types.CategoryExploration, true
	}
	if hasTasks && !hasEdits {
		return types.CategoryPlanning, true
	}
	if hasSkill {
		return types.CategoryGeneral, true
	}

	return "", false
}

func refineByKeywords(category types.TaskCategory, userMessage string) types.TaskCategory {
	if category == types.CategoryCoding {
		if debugKeywords.MatchString(userMessage) {
			return types.CategoryDebugging
		}
		if refactorKeywords.MatchString(userMessage) {
			return types.CategoryRefactoring
		}
		if featureKeywords.MatchString(userMessage) {
			return types.CategoryFeature
		}
		return types.CategoryCoding
	}

	if category == types.CategoryExploration {
		if researchKeywords.MatchString(userMessage) {
			return types.CategoryExploration
		}
		if debugKeywords.MatchString(userMessage) {
			return types.CategoryDebugging
		}
		return types.CategoryExploration
	}

	return category
}

func classifyConversation(userMessage string) types.TaskCategory {
	if brainstormKeywords.MatchString(userMessage) {
		return types.CategoryBrainstorm
	}
	if researchKeywords.MatchString(userMessage) {
		return types.CategoryExploration
	}
	if debugKeywords.MatchString(userMessage) {
		return types.CategoryDebugging
	}
	if featureKeywords.MatchString(userMessage) {
		return types.CategoryFeature
	}
	if filePatterns.MatchString(userMessage) {
		return types.CategoryCoding
	}
	if scriptPatterns.MatchString(userMessage) {
		return types.CategoryCoding
	}
	if urlPattern.MatchString(userMessage) {
		return types.CategoryExploration
	}
	return types.CategoryConversation
}

func countRetries(turn types.ParsedTurn) int {
	sawEditBeforeBash := false
	sawBashAfterEdit := false
	retries := 0

	for _, call := range turn.AssistantCalls {
		hasEdit := false
		for _, t := range call.Tools {
			if inSet(t, editTools) {
				hasEdit = true
				break
			}
		}
		hasBash := false
		for _, t := range call.Tools {
			if inSet(t, BashTools) {
				hasBash = true
				break
			}
		}

		if hasEdit {
			if sawBashAfterEdit {
				retries++
			}
			sawEditBeforeBash = true
			sawBashAfterEdit = false
		}
		if hasBash && sawEditBeforeBash {
			sawBashAfterEdit = true
		}
	}

	return retries
}

func turnHasEdits(turn types.ParsedTurn) bool {
	for _, call := range turn.AssistantCalls {
		for _, t := range call.Tools {
			if inSet(t, editTools) {
				return true
			}
		}
	}
	return false
}

// ClassifyTurn classifies a parsed turn and returns a ClassifiedTurn with
// category, retry count, and edit flag populated.
func ClassifyTurn(turn types.ParsedTurn) types.ClassifiedTurn {
	tools := getAllTools(turn)

	var category types.TaskCategory

	if len(tools) == 0 {
		category = classifyConversation(turn.UserMessage)
	} else {
		toolCategory, ok := classifyByToolPattern(turn)
		if ok {
			category = refineByKeywords(toolCategory, turn.UserMessage)
		} else {
			category = classifyConversation(turn.UserMessage)
		}
	}

	return types.ClassifiedTurn{
		ParsedTurn: turn,
		Category:   category,
		Retries:    countRetries(turn),
		HasEdits:   turnHasEdits(turn),
	}
}
