// httpclient/sse_test.go
package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sseServer(t *testing.T, events []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		require.True(t, ok, "ResponseWriter must implement http.Flusher")
		for _, e := range events {
			fmt.Fprint(w, e)
			flusher.Flush()
		}
	}))
}

func TestExecuteSSE_receivesEventsWithCorrectFields(t *testing.T) {
	srv := sseServer(t, []string{
		"event: update\ndata: hello\n\n",
		"data: world\n\n",
	})
	defer srv.Close()

	var received []SSEEvent
	err := New().Get(srv.URL).ExecuteSSE(func(e SSEEvent) error {
		received = append(received, e)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, received, 2)
	assert.Equal(t, "update", received[0].Event)
	assert.Equal(t, "hello", received[0].Data)
	assert.Equal(t, "message", received[1].Event) // default when no event field
	assert.Equal(t, "world", received[1].Data)
}

func TestExecuteSSE_callbackErrorStopsStream(t *testing.T) {
	srv := sseServer(t, []string{
		"data: first\n\n",
		"data: second\n\n",
	})
	defer srv.Close()

	count := 0
	err := New().Get(srv.URL).ExecuteSSE(func(e SSEEvent) error {
		count++
		return fmt.Errorf("stop after first")
	})
	assert.Error(t, err)
	assert.Equal(t, 1, count)
}

func TestExecuteSSE_contextCancellationStopsStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		for i := 0; i < 100; i++ {
			fmt.Fprintf(w, "data: msg%d\n\n", i)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	count := 0
	err := New().Get(srv.URL).WithContext(ctx).ExecuteSSE(func(e SSEEvent) error {
		count++
		return nil
	})
	assert.Error(t, err)
	assert.Less(t, count, 100)
}

func TestParseSSELine_parsesAllFieldTypes(t *testing.T) {
	tests := []struct {
		input   string
		wantKey string
		wantVal string
	}{
		{"data: hello", "data", "hello"},
		{"event: update", "event", "update"},
		{"id: 42", "id", "42"},
		{"retry: 3000", "retry", "3000"},
		{": comment", "", "comment"},
		{"", "", ""},
		{"data:", "data", ""},
	}
	for _, tt := range tests {
		key, val := parseSSELine(tt.input)
		assert.Equal(t, tt.wantKey, key, "input: %q", tt.input)
		assert.Equal(t, tt.wantVal, val, "input: %q", tt.input)
	}
}

func TestSSEStream_defaultEventNameIsMessage(t *testing.T) {
	r := strings.NewReader("data: test\n\n")
	var events []SSEEvent
	parseSSEStream(r, func(e SSEEvent) error {
		events = append(events, e)
		return nil
	})
	require.Len(t, events, 1)
	assert.Equal(t, "message", events[0].Event)
	assert.Equal(t, "test", events[0].Data)
}

func TestSSEStream_multilineData_joinedWithNewline(t *testing.T) {
	r := strings.NewReader("data: line1\ndata: line2\n\n")
	var events []SSEEvent
	parseSSEStream(r, func(e SSEEvent) error {
		events = append(events, e)
		return nil
	})
	require.Len(t, events, 1)
	assert.Equal(t, "line1\nline2", events[0].Data)
}
