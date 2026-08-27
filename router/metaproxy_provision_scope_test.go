package router

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// metaproxy requests this scope on its access token; changing it breaks
// provisioning until the token is reissued.
func TestMetaproxyProvisionRequiresDedicatedRootScope(t *testing.T) {
	rule, ok := middleware.AccessTokenRouteRule("POST /api/metaproxy/provision")
	require.True(t, ok)
	assert.Equal(t, "scope", rule.Kind())
	assert.Equal(t, "metaproxy:write", rule.Scope())

	grantable := func(role int) bool {
		for _, group := range service.AccessTokenCatalog(1, role) {
			for _, resource := range group.Resources {
				for _, action := range resource.Actions {
					if resource.Resource+":"+action.Action == "metaproxy:write" {
						return true
					}
				}
			}
		}
		return false
	}
	assert.True(t, grantable(common.RoleRootUser))
	assert.False(t, grantable(common.RoleAdminUser))
}
