package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenPatchPreservesUnspecifiedFields(t *testing.T) {
	initModelListColumnNames(t)
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "patch-owner", Group: "default"}).Error)
	previous := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() { require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previous)) })
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default","claude-plus":"Claude Plus"}`))
	ips := "127.0.0.1"
	token := model.Token{UserId: 1, Key: "patch-test-secret", Name: "Keep name", Status: common.TokenStatusEnabled,
		ExpiredTime: 2000000000, RemainQuota: 9000, UsedQuota: 1000, Group: "default",
		ModelLimitsEnabled: true, ModelLimits: "claude-opus-5", AllowIps: &ips, CrossGroupRetry: true}
	require.NoError(t, db.Create(&token).Error)
	router := gin.New()
	router.PATCH("/token", func(c *gin.Context) { c.Set("id", 1); UpdateToken(c) })
	for _, test := range []struct {
		name    string
		body    string
		success bool
	}{
		{"change group", `"group":"claude-plus"`, true},
		{"reject unavailable group", `"group":"private"`, false},
		{"reject protected quota counter", `"used_quota":0`, false},
		{"reject null", `"name":null`, false},
		{"reject invalid status", `"status":0`, false},
		{"reject negative quota", `"unlimited_quota":true,"remain_quota":-1`, false},
		{"explicit false and empty", `"model_limits_enabled":false,"model_limits":"","cross_group_retry":false`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPatch, "/token", bytes.NewBufferString(fmt.Sprintf(`{"id":%d,%s}`, token.Id, test.body))))
			var response tokenAPIResponse
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
			assert.Equal(t, test.success, response.Success, recorder.Body.String())
			stored, err := model.GetTokenByIds(token.Id, 1)
			require.NoError(t, err)
			assert.Equal(t, "claude-plus", stored.Group)
			assert.Equal(t, token.Name, stored.Name)
			assert.Equal(t, token.RemainQuota, stored.RemainQuota)
			assert.Equal(t, token.UsedQuota, stored.UsedQuota)
			assert.Equal(t, token.ExpiredTime, stored.ExpiredTime)
			assert.Equal(t, token.AllowIps, stored.AllowIps)
			if test.name == "explicit false and empty" {
				assert.False(t, stored.ModelLimitsEnabled)
				assert.Empty(t, stored.ModelLimits)
				assert.False(t, stored.CrossGroupRetry)
			}
		})
	}
	other := model.Token{UserId: 2, Key: "other-owner", Name: "Other", Group: "default"}
	require.NoError(t, db.Create(&other).Error)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPatch, "/token", bytes.NewBufferString(fmt.Sprintf(`{"id":%d,"group":"claude-plus"}`, other.Id))))
	var response tokenAPIResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.False(t, response.Success)
	stored, err := model.GetTokenByIds(other.Id, 2)
	require.NoError(t, err)
	assert.Equal(t, "default", stored.Group)
}
