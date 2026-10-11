package reasoning

import "strings"

// ClaudeAdaptiveOnly reports Claude models that reject thinking.type="enabled"
// with budget_tokens and non-default temperature/top_p/top_k. Their thinking
// is adaptive and its strength is steered by output_config.effort.
func ClaudeAdaptiveOnly(model string) bool {
	model = strings.ToLower(model)
	for _, prefix := range []string{"claude-opus-4-7", "claude-opus-4-8", "claude-opus-5", "claude-sonnet-5", "claude-haiku-5"} {
		if strings.HasPrefix(model, prefix) {
			return true
		}
	}
	return false
}
