package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscriptionCurrency(t *testing.T) {
	for _, tc := range []struct {
		name     string
		plan     model.SubscriptionPlan
		currency string
		invalid  bool
	}{
		{"legacy default", model.SubscriptionPlan{}, "USD", false},
		{"CNY Epay", model.SubscriptionPlan{Currency: " cny ", AllowBalancePay: common.GetPointer(false)}, "CNY", false},
		{"CNY implicit balance", model.SubscriptionPlan{Currency: "CNY"}, "", true},
		{"CNY balance", model.SubscriptionPlan{Currency: "CNY", AllowBalancePay: common.GetPointer(true)}, "", true},
		{"CNY Stripe", model.SubscriptionPlan{Currency: "CNY", AllowBalancePay: common.GetPointer(false), StripePriceId: "price"}, "", true},
		{"unknown", model.SubscriptionPlan{Currency: "EUR"}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := normalizeSubscriptionCurrency(&tc.plan)
			if tc.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.currency, tc.plan.Currency)
		})
	}
}
