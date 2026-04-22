package classifier

import (
	"testing"

	"github.com/agentseal/codeburn/internal/types"
)

func makeTurn(userMsg string, toolSets ...[]string) types.ParsedTurn {
	turn := types.ParsedTurn{UserMessage: userMsg}
	for _, tools := range toolSets {
		turn.AssistantCalls = append(turn.AssistantCalls, types.ParsedApiCall{Tools: tools})
	}
	return turn
}

func TestClassifyTurn_Coding(t *testing.T) {
	turn := makeTurn("update the handler", []string{"Edit"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryCoding {
		t.Errorf("expected coding, got %q", result.Category)
	}
}

func TestClassifyTurn_Debugging(t *testing.T) {
	turn := makeTurn("fix the crash in auth", []string{"Edit"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryDebugging {
		t.Errorf("expected debugging, got %q", result.Category)
	}
}

func TestClassifyTurn_Feature(t *testing.T) {
	turn := makeTurn("add a new endpoint for payments", []string{"Write"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryFeature {
		t.Errorf("expected feature, got %q", result.Category)
	}
}

func TestClassifyTurn_Refactoring(t *testing.T) {
	turn := makeTurn("refactor the database module", []string{"Edit"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryRefactoring {
		t.Errorf("expected refactoring, got %q", result.Category)
	}
}

func TestClassifyTurn_Testing(t *testing.T) {
	turn := makeTurn("run vitest", []string{"Bash"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryTesting {
		t.Errorf("expected testing, got %q", result.Category)
	}
}

func TestClassifyTurn_Git(t *testing.T) {
	turn := makeTurn("git push origin main", []string{"Bash"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryGit {
		t.Errorf("expected git, got %q", result.Category)
	}
}

func TestClassifyTurn_BuildDeploy_Build(t *testing.T) {
	turn := makeTurn("npm run build", []string{"Bash"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryBuildDeploy {
		t.Errorf("expected build/deploy, got %q", result.Category)
	}
}

func TestClassifyTurn_BuildDeploy_Install(t *testing.T) {
	turn := makeTurn("npm install lodash", []string{"Bash"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryBuildDeploy {
		t.Errorf("expected build/deploy, got %q", result.Category)
	}
}

func TestClassifyTurn_Exploration_ReadOnly(t *testing.T) {
	turn := makeTurn("look at the config", []string{"Read", "Grep"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryExploration {
		t.Errorf("expected exploration, got %q", result.Category)
	}
}

func TestClassifyTurn_Exploration_BashRead(t *testing.T) {
	turn := makeTurn("check the logs", []string{"Bash", "Read"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryExploration {
		t.Errorf("expected exploration, got %q", result.Category)
	}
}

func TestClassifyTurn_Exploration_Search(t *testing.T) {
	turn := makeTurn("find docs online", []string{"WebSearch"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryExploration {
		t.Errorf("expected exploration, got %q", result.Category)
	}
}

func TestClassifyTurn_Planning_Tasks(t *testing.T) {
	turn := makeTurn("plan the migration", []string{"TodoWrite"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryPlanning {
		t.Errorf("expected planning, got %q", result.Category)
	}
}

func TestClassifyTurn_Planning_PlanMode(t *testing.T) {
	turn := types.ParsedTurn{
		UserMessage: "plan the feature",
		AssistantCalls: []types.ParsedApiCall{
			{Tools: []string{"Read"}, HasPlanMode: true},
		},
	}
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryPlanning {
		t.Errorf("expected planning, got %q", result.Category)
	}
}

func TestClassifyTurn_Delegation(t *testing.T) {
	turn := types.ParsedTurn{
		UserMessage: "delegate this",
		AssistantCalls: []types.ParsedApiCall{
			{Tools: []string{"Bash"}, HasAgentSpawn: true},
		},
	}
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryDelegation {
		t.Errorf("expected delegation, got %q", result.Category)
	}
}

func TestClassifyTurn_General_Skill(t *testing.T) {
	turn := makeTurn("run the skill", []string{"Skill"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryGeneral {
		t.Errorf("expected general, got %q", result.Category)
	}
}

// Conversation path (no tools)

func TestClassifyTurn_Conversation(t *testing.T) {
	turn := makeTurn("hello there")
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryConversation {
		t.Errorf("expected conversation, got %q", result.Category)
	}
}

func TestClassifyTurn_Brainstorming(t *testing.T) {
	turn := makeTurn("brainstorm ideas for the API design")
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryBrainstorm {
		t.Errorf("expected brainstorming, got %q", result.Category)
	}
}

func TestClassifyTurn_Exploration_Conversation(t *testing.T) {
	turn := makeTurn("research how OAuth2 works")
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryExploration {
		t.Errorf("expected exploration, got %q", result.Category)
	}
}

func TestClassifyTurn_Debugging_Conversation(t *testing.T) {
	turn := makeTurn("why does the 500 error happen")
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryDebugging {
		t.Errorf("expected debugging, got %q", result.Category)
	}
}

func TestClassifyTurn_Feature_Conversation(t *testing.T) {
	turn := makeTurn("create a new login page")
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryFeature {
		t.Errorf("expected feature, got %q", result.Category)
	}
}

func TestClassifyTurn_Coding_FilePattern(t *testing.T) {
	turn := makeTurn("update config.json please")
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryCoding {
		t.Errorf("expected coding, got %q", result.Category)
	}
}

func TestClassifyTurn_Coding_ScriptPattern(t *testing.T) {
	turn := makeTurn("execute the migration script")
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryCoding {
		t.Errorf("expected coding, got %q", result.Category)
	}
}

func TestClassifyTurn_Exploration_URL(t *testing.T) {
	turn := makeTurn("check https://example.com/docs")
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryExploration {
		t.Errorf("expected exploration, got %q", result.Category)
	}
}

// Retry counting

func TestCountRetries_None(t *testing.T) {
	turn := makeTurn("do something", []string{"Edit"})
	result := ClassifyTurn(turn)
	if result.Retries != 0 {
		t.Errorf("expected 0 retries, got %d", result.Retries)
	}
}

func TestCountRetries_One(t *testing.T) {
	// Edit -> Bash -> Edit = 1 retry
	turn := types.ParsedTurn{
		UserMessage: "fix it",
		AssistantCalls: []types.ParsedApiCall{
			{Tools: []string{"Edit"}},
			{Tools: []string{"Bash"}},
			{Tools: []string{"Edit"}},
		},
	}
	result := ClassifyTurn(turn)
	if result.Retries != 1 {
		t.Errorf("expected 1 retry, got %d", result.Retries)
	}
}

func TestCountRetries_Two(t *testing.T) {
	// Edit -> Bash -> Edit -> Bash -> Edit = 2 retries
	turn := types.ParsedTurn{
		UserMessage: "fix it",
		AssistantCalls: []types.ParsedApiCall{
			{Tools: []string{"Edit"}},
			{Tools: []string{"Bash"}},
			{Tools: []string{"Edit"}},
			{Tools: []string{"Bash"}},
			{Tools: []string{"Edit"}},
		},
	}
	result := ClassifyTurn(turn)
	if result.Retries != 2 {
		t.Errorf("expected 2 retries, got %d", result.Retries)
	}
}

func TestCountRetries_BashOnlyNoEdit(t *testing.T) {
	turn := makeTurn("run tests", []string{"Bash"})
	result := ClassifyTurn(turn)
	if result.Retries != 0 {
		t.Errorf("expected 0 retries, got %d", result.Retries)
	}
}

// HasEdits flag

func TestHasEdits_True(t *testing.T) {
	turn := makeTurn("edit the file", []string{"Edit"})
	result := ClassifyTurn(turn)
	if !result.HasEdits {
		t.Error("expected HasEdits to be true")
	}
}

func TestHasEdits_Write(t *testing.T) {
	turn := makeTurn("write a file", []string{"Write"})
	result := ClassifyTurn(turn)
	if !result.HasEdits {
		t.Error("expected HasEdits to be true for Write tool")
	}
}

func TestHasEdits_False(t *testing.T) {
	turn := makeTurn("read the file", []string{"Read"})
	result := ClassifyTurn(turn)
	if result.HasEdits {
		t.Error("expected HasEdits to be false")
	}
}

func TestHasEdits_NoTools(t *testing.T) {
	turn := makeTurn("just chatting")
	result := ClassifyTurn(turn)
	if result.HasEdits {
		t.Error("expected HasEdits to be false with no tools")
	}
}

// BashTools export

func TestBashToolsExported(t *testing.T) {
	expected := []string{"Bash", "BashTool", "PowerShellTool"}
	for _, tool := range expected {
		if _, ok := BashTools[tool]; !ok {
			t.Errorf("BashTools missing %q", tool)
		}
	}
	if len(BashTools) != len(expected) {
		t.Errorf("BashTools has %d entries, expected %d", len(BashTools), len(expected))
	}
}

// MCP tool detection

func TestClassifyTurn_Exploration_MCP(t *testing.T) {
	turn := makeTurn("use the mcp tool", []string{"mcp__some_server__some_tool"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryExploration {
		t.Errorf("expected exploration for mcp tool, got %q", result.Category)
	}
}

// Exploration refinement: debug keywords on exploration turn

func TestRefineByKeywords_ExplorationToDebugging(t *testing.T) {
	// Read tools -> exploration, but "error" in message -> debugging
	turn := makeTurn("find the error in logs", []string{"Read"})
	result := ClassifyTurn(turn)
	if result.Category != types.CategoryDebugging {
		t.Errorf("expected debugging after refinement, got %q", result.Category)
	}
}

// Bash+Edit combo should be coding (edit wins), not testing even if test keyword present

func TestClassifyTurn_BashAndEdit_IsCoding(t *testing.T) {
	turn := makeTurn("run vitest after editing", []string{"Edit", "Bash"})
	result := ClassifyTurn(turn)
	// hasEdits=true, so coding path is taken, then refined
	if result.Category == types.CategoryTesting {
		t.Errorf("expected non-testing category when edit tools present, got %q", result.Category)
	}
}
