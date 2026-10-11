package reasoning

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveClaudeDefaultLeavesHaiku5WithoutIntentUnchanged(t *testing.T) {
	t.Parallel()

	require.Equal(t, Intent{}, ResolveClaudeDefault("claude-haiku-5-5", Intent{}))
	require.Equal(t, ModeAdaptive, ResolveClaudeDefault("claude-sonnet-5", Intent{}).Mode)
}
