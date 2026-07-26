package dream

import (
	"context"
	"os"
	"strings"
	"testing"
)

// fakeHookGen is the injected seam that makes the dream job hermetic — it
// returns a fixed hook or a fixed error, never touching the network.
type fakeHookGen struct {
	hook string
	err  error
}

func (f fakeHookGen) Hook(context.Context, string, string) (string, error) {
	return f.hook, f.err
}

var _ HookGen = fakeHookGen{}
var _ HookGen = (*llmHookGen)(nil)

func TestHookSystemPromptForbidsStructure(t *testing.T) {
	for _, forbid := range []string{"No markdown", "no newline", "no leading dash", "ONLY the phrase"} {
		if !strings.Contains(hookSystemPrompt, forbid) {
			t.Fatalf("system prompt missing guard %q", forbid)
		}
	}
	if strings.Contains(hookSystemPrompt, "|") {
		t.Fatal("system prompt itself must not contain the index delimiter")
	}
}

func TestConfigFromEnvDefaults(t *testing.T) {
	for _, k := range []string{"MWL_DATA_ROOT", "MWL_DB_PATH", "LLM_BASE_URL", "LLM_API_KEY", "DREAM_MODEL", "T1", "T2", "T3", "GRACE", "DREAM_MAX_DELETIONS"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	c := FromEnv()
	if c.T1 != 14*day || c.T2 != 90*day || c.T3 != 180*day || c.Grace != 30*day {
		t.Fatalf("threshold defaults wrong: %+v", c)
	}
	if c.MaxDeletions != 50 {
		t.Fatalf("MaxDeletions default = %d, want 50", c.MaxDeletions)
	}
	if c.LLMBaseURL != "https://api.deepseek.com/v1" {
		t.Fatalf("LLMBaseURL default = %q", c.LLMBaseURL)
	}
	if c.DataRoot != "./data" || c.DBPath != "data/mywholelife.db" {
		t.Fatalf("data defaults wrong: %q %q", c.DataRoot, c.DBPath)
	}
}

func TestConfigFromEnvOverrides(t *testing.T) {
	t.Setenv("T1", "1")
	t.Setenv("T2", "2")
	t.Setenv("T3", "3")
	t.Setenv("GRACE", "4")
	t.Setenv("DREAM_MAX_DELETIONS", "7")
	t.Setenv("LLM_BASE_URL", "https://example.test/v1")
	t.Setenv("DREAM_MODEL", "qwen-max")
	c := FromEnv()
	if c.T1 != 1*day || c.T2 != 2*day || c.T3 != 3*day || c.Grace != 4*day {
		t.Fatalf("day overrides not converted to seconds: %+v", c)
	}
	if c.MaxDeletions != 7 || c.LLMBaseURL != "https://example.test/v1" || c.Model != "qwen-max" {
		t.Fatalf("overrides not applied: %+v", c)
	}
}
