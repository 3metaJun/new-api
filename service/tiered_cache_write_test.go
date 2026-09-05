package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTieredUnsplitClaudeCacheWrites(t *testing.T) {
	usage := &dto.Usage{PromptTokens: 2000, CompletionTokens: 1000, UsageSemantic: "anthropic"}
	usage.PromptTokensDetails.CachedTokens = 8000
	usage.PromptTokensDetails.CachedCreationTokens = 1000
	params := BuildTieredTokenParams(usage, true, map[string]bool{"p": true, "c": true, "cr": true, "cc": true})
	assert.Equal(t, float64(1000), params.CC)
	assert.Equal(t, float64(11000), params.Len)
	cost, _, err := billingexpr.RunExpr(`p * 19 + c * 94 + cr * 1.9 + cc * 23.75`, params)
	require.NoError(t, err)
	assert.InDelta(t, 170950, cost, 0.000001)

	usage.ClaudeCacheCreation1hTokens = 400
	params = BuildTieredTokenParams(usage, true, map[string]bool{"p": true, "c": true, "cr": true, "cc": true, "cc1h": true})
	assert.Equal(t, float64(600), params.CC)
	assert.Equal(t, float64(400), params.CC1h)
	assert.Equal(t, float64(11000), params.Len)

	params = BuildTieredTokenParams(usage, true, map[string]bool{"p": true, "c": true})
	assert.Equal(t, float64(11000), params.P)
	assert.Equal(t, float64(11000), params.Len)
}
