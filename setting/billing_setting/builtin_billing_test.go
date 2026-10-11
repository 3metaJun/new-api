package billing_setting_test

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetBillingSettings(t *testing.T) *billing_setting.BillingSetting {
	t.Helper()
	settings := config.GlobalConfig.Get("billing_setting").(*billing_setting.BillingSetting)
	saved := *settings
	savedRatios, savedPrices := ratio_setting.ModelRatio2JSONString(), ratio_setting.ModelPrice2JSONString()
	t.Cleanup(func() {
		*settings = saved
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedRatios))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedPrices))
	})
	*settings = billing_setting.BillingSetting{BillingMode: map[string]string{}, BillingExpr: map[string]string{}}
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{}`))
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{}`))
	return settings
}

func TestBuiltinBillingListPrices(t *testing.T) {
	resetBillingSettings(t)

	for _, tc := range []struct {
		name, model                    string
		input, output, cached, written int
		quota                          int
	}{
		// 1M tokens bill at the listed USD price; QuotaPerUnit 500000 halves the per-million sum.
		{"gpt-6-luna list price", "gpt-6-luna", 1000000, 1000000, 0, 0, 300000},
		{"gpt-6-luna cache read and write", "gpt-6-luna", 1000000, 0, 600000, 200000, 25500},
		{"gemini-3.8-flash list price", "gemini-3.8-flash", 1000000, 1000000, 0, 0, 2250000},
		{"gemini-3.8-flash cache read", "gemini-3.8-flash", 1000000, 0, 800000, 0, 105000},
		{"claude-haiku-5-5 at the 100K boundary", "claude-haiku-5-5", 100000, 10000, 0, 0, 7500},
		{"claude-haiku-5-5 above 100K bills the whole request at 5x", "claude-haiku-5-5", 100001, 10000, 0, 0, 37500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, billing_setting.BillingModeTieredExpr, billing_setting.GetBillingMode(tc.model))
			expression, ok := billing_setting.GetBillingExpr(tc.model)
			require.True(t, ok)
			usage := &dto.Usage{
				PromptTokens: tc.input, CompletionTokens: tc.output,
				PromptTokensDetails: dto.InputTokenDetails{CachedTokens: tc.cached, CacheWriteTokens: tc.written},
			}
			params := service.BuildTieredTokenParams(usage, false, billingexpr.UsedVars(expression))
			result, err := billingexpr.ComputeTieredQuotaWithRequest(&billingexpr.BillingSnapshot{
				ExprString: expression, ExprHash: billingexpr.ExprHashString(expression), GroupRatio: 1, QuotaPerUnit: 500000,
			}, params, billingexpr.RequestInput{Body: []byte(`{}`)})
			require.NoError(t, err)
			assert.Equal(t, tc.quota, result.ActualQuotaAfterGroup)
		})
	}
}

func TestBuiltinBillingYieldsToAdministratorSettings(t *testing.T) {
	settings := resetBillingSettings(t)
	const model = "gemini-3.8-flash"

	for _, tc := range []struct {
		name, mode, expr, ratios, prices, wantMode string
	}{
		{"custom expression overrides the built-in one", "tiered_expr", "p * 7", `{}`, `{}`, "tiered_expr"},
		{"explicit ratio mode", "ratio", "", `{}`, `{}`, "ratio"},
		{"existing token ratio", "", "", `{"gemini-3.8-flash":0}`, `{}`, "ratio"},
		{"existing per-call price", "", "", `{}`, `{"gemini-3.8-flash":0.1}`, "ratio"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			*settings = billing_setting.BillingSetting{BillingMode: map[string]string{}, BillingExpr: map[string]string{}}
			if tc.mode != "" {
				settings.BillingMode[model] = tc.mode
			}
			if tc.expr != "" {
				settings.BillingExpr[model] = tc.expr
			}
			require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(tc.ratios))
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(tc.prices))
			assert.Equal(t, tc.wantMode, billing_setting.GetBillingMode(model))
			actual, ok := billing_setting.GetBillingExpr(model)
			assert.Equal(t, tc.expr, actual)
			assert.Equal(t, tc.expr != "", ok)
		})
	}
}
