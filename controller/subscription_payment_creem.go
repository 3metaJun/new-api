package controller

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/thanhpk/randstr"
)

type SubscriptionCreemPayRequest struct {
	PlanId int `json:"plan_id"`
}

func SubscriptionRequestCreemPay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req SubscriptionCreemPayRequest

	// Keep body for debugging consistency (like RequestCreemPay)
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logger.LogError(c.Request.Context(), common.LogText("Creem failed to read subscription payment request error=%q", err.Error()))
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "read query error"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		paymentError(c, common.NewMessage("Invalid parameters"))
		return
	}

	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorT(c, "Subscription plan is not enabled")
		return
	}
	if plan.CreemProductId == "" {
		common.ApiErrorT(c, "CreemProductId is not configured for this plan")
		return
	}
	config := currentCreemConfig()
	if config.ApiKey == "" || config.WebhookSecret == "" {
		common.ApiErrorT(c, "Creem webhook is not configured")
		return
	}

	userId := c.GetInt("id")
	if model.NormalizeResetPeriod(plan.QuotaResetPeriod) != model.SubscriptionResetNever || plan.MaxPurchasePerUser != 0 {
		common.ApiErrorMsg(c, "Creem recurring plans must use quota_reset_period=never and max_purchase_per_user=0")
		return
	}
	if err := model.CheckActiveSubscriptionLimit(userId, plan); err != nil {
		common.ApiError(c, err)
		return
	}
	user, err := model.GetUserById(userId, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if user == nil {
		common.ApiErrorT(c, "User does not exist")
		return
	}

	if plan.MaxPurchasePerUser > 0 {
		count, err := model.CountUserSubscriptionsByPlan(userId, plan.Id)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			common.ApiErrorT(c, "Purchase limit for this plan has been reached")
			return
		}
	}

	reference := "sub-creem-ref-" + randstr.String(6)
	referenceId := "sub_ref_" + common.Sha1([]byte(reference+time.Now().String()+user.Username))

	allowWalletOverflow := true
	if plan.AllowWalletOverflow != nil {
		allowWalletOverflow = *plan.AllowWalletOverflow
	}

	// Create the pending order with an immutable provider contract snapshot.
	order := &model.SubscriptionOrder{
		UserId:              userId,
		PlanId:              plan.Id,
		Money:               plan.PriceAmount,
		TradeNo:             referenceId,
		PaymentMethod:       model.PaymentMethodCreem,
		PaymentProvider:     model.PaymentProviderCreem,
		CreateTime:          time.Now().Unix(),
		Status:              common.TopUpStatusPending,
		ContractSnapshot:    true,
		ProviderProductId:   plan.CreemProductId,
		Currency:            plan.Currency,
		PlanTitle:           plan.Title,
		AmountTotal:         plan.TotalAmount,
		WeeklyAmount:        plan.WeeklyAmount,
		MaxActivePerUser:    plan.MaxActivePerUser,
		AllowWalletOverflow: allowWalletOverflow,
		UpgradeGroup:        plan.UpgradeGroup,
		DowngradeGroup:      plan.DowngradeGroup,
	}
	if err := model.ReserveCreemSubscriptionCheckout(userId, order); err != nil {
		if errors.Is(err, model.ErrCreemCheckoutAlreadyPending) {
			common.ApiErrorMsg(c, "A Creem subscription checkout is already pending")
			return
		}
		paymentError(c, common.NewMessage("Failed to create order"))
		return
	}

	// Reuse Creem checkout generator by building a lightweight product reference.
	product := &CreemProduct{
		ProductId: plan.CreemProductId,
		Name:      plan.Title,
		Price:     plan.PriceAmount,
		Currency:  plan.Currency,
		Quota:     0,
	}

	checkoutUrl, err := genCreemLink(c.Request.Context(), referenceId, product, user.Email, user.Username, config)
	if err != nil {
		_ = model.ReleaseCreemSubscriptionCheckout(referenceId)
		logger.LogError(c.Request.Context(), common.LogText("Creem failed to create subscription payment link trade_no=%s product_id=%s error=%q", referenceId, product.ProductId, err.Error()))
		paymentError(c, common.NewMessage("Failed to start payment"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "success",
		"data": gin.H{
			"checkout_url": checkoutUrl,
			"order_id":     referenceId,
		},
	})
}
