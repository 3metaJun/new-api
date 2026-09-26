package channel

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setStreamFirstResponseTimeout(t *testing.T, seconds int, perModel map[string]int) {
	t.Helper()
	setting := operation_setting.GetGeneralSetting()
	previousSeconds, previousPerModel := setting.StreamFirstResponseTimeoutSeconds, setting.StreamFirstResponseTimeoutModelSeconds
	setting.StreamFirstResponseTimeoutSeconds = seconds
	setting.StreamFirstResponseTimeoutModelSeconds = perModel
	t.Cleanup(func() {
		setting.StreamFirstResponseTimeoutSeconds = previousSeconds
		setting.StreamFirstResponseTimeoutModelSeconds = previousPerModel
	})
}

func doStreamRequest(t *testing.T, url string, model string) (*http.Response, error) {
	t.Helper()
	service.InitHttpClient()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader("{}"))
	require.NoError(t, err)
	info := &relaycommon.RelayInfo{IsStream: true, OriginModelName: model, ChannelMeta: &relaycommon.ChannelMeta{}}
	return doRequest(ctx, req, info)
}

func TestStreamFirstResponseTimeoutPerModelOverride(t *testing.T) {
	setStreamFirstResponseTimeout(t, 20, map[string]int{"reasoning-model": 90, "exempt-model": 0})

	assert.Equal(t, 20*time.Second, operation_setting.StreamFirstResponseTimeout("chat-model"))
	assert.Equal(t, 90*time.Second, operation_setting.StreamFirstResponseTimeout("reasoning-model"))
	assert.Equal(t, time.Duration(0), operation_setting.StreamFirstResponseTimeout("exempt-model"))
}

func TestDoRequestAbortsStreamWithoutDataBeforeDeadline(t *testing.T) {
	setStreamFirstResponseTimeout(t, 1, map[string]int{})
	cases := map[string]http.HandlerFunc{
		"no response headers": func(w http.ResponseWriter, r *http.Request) {
			// Reading the body lets the server notice the client hanging up and cancel r.Context().
			_, _ = io.ReadAll(r.Body)
			<-r.Context().Done()
		},
		// A keep-alive comment is not data: a relay can emit these while its own upstream is stuck.
		"headers and keep-alive only": func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, ": keep-alive\n\n")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		},
	}
	for name, handler := range cases {
		t.Run(name, func(t *testing.T) {
			upstream := httptest.NewServer(handler)
			defer upstream.Close()

			started := time.Now()
			resp, err := doStreamRequest(t, upstream.URL, "stalled-model")

			require.Error(t, err)
			assert.Nil(t, resp)
			assert.Contains(t, err.Error(), "upstream sent no stream data within 1s")
			assert.Less(t, time.Since(started), 5*time.Second)
		})
	}
}

func TestDoRequestKeepsStreamBytesAfterFirstData(t *testing.T) {
	setStreamFirstResponseTimeout(t, 1, map[string]int{})
	const head = ": keep-alive\n\nevent: message\ndata: {\"id\":1}\n\n"
	const tail = "data: [DONE]\n\n"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, head)
		w.(http.Flusher).Flush()
		// Silence after the first data line is the stream handler's business, not the deadline's.
		time.Sleep(1500 * time.Millisecond)
		_, _ = io.WriteString(w, tail)
	}))
	defer upstream.Close()

	resp, err := doStreamRequest(t, upstream.URL, "streaming-model")
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, head+tail, string(body))
}
