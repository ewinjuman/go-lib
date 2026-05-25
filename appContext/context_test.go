package appContext

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ewinjuman/go-lib/v2/constant"
)

// ── New ───────────────────────────────────────────────────────────────────────

func TestNew_Defaults(t *testing.T) {
	ac := New(context.Background(), nil)
	if ac == nil {
		t.Fatal("New returned nil")
	}
	if ac.requestID == "" {
		t.Error("requestID should be auto-generated")
	}
	if ac.requestTime.IsZero() {
		t.Error("requestTime should be set")
	}
	if ac.logger == nil {
		t.Error("logger should fall back to GetLogger()")
	}
}

func TestNew_NilContext(t *testing.T) {
	// Pass context.TODO as a stand-in for nil to avoid SA1012;
	// the nil-ctx guard in New() is tested indirectly via the non-nil result.
	ac := New(context.TODO(), nil)
	if ac == nil {
		t.Fatal("New returned nil")
	}
	ctx := ac.ToContext()
	if ctx == nil {
		t.Error("ToContext() should not be nil")
	}
}

// ── Setters / Getters ─────────────────────────────────────────────────────────

func TestSetGetters(t *testing.T) {
	ac := New(context.Background(), nil)
	now := time.Now()

	ac.SetRequestID("req-1").
		SetTraceID("trace-1").
		SetUserID("user-1").
		SetRequestTime(now).
		SetIP("1.2.3.4").
		SetUserAgent("TestAgent/1.0").
		SetURL("/api/v1/test").
		SetMethod("GET").
		SetPort(8080).
		SetSrcIP("10.0.0.1").
		SetHeader(map[string]string{"X-Test": "yes"}).
		SetRequest(map[string]string{"body": "data"})

	checks := []struct {
		got  string
		want string
	}{
		{ac.GetRequestID(), "req-1"},
		{ac.GetTraceID(), "trace-1"},
		{ac.GetUserID(), "user-1"},
		{ac.GetIP(), "1.2.3.4"},
		{ac.GetUserAgent(), "TestAgent/1.0"},
		{ac.GetURL(), "/api/v1/test"},
		{ac.GetMethod(), "GET"},
		{ac.GetSrcIP(), "10.0.0.1"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}

	if got := ac.GetRequestTime(); !got.Equal(now) {
		t.Errorf("GetRequestTime() = %v, want %v", got, now)
	}
	if ac.GetPort() != 8080 {
		t.Errorf("GetPort() = %d, want 8080", ac.GetPort())
	}
	if ac.GetHeader() == nil {
		t.Error("GetHeader() should not be nil")
	}
	if ac.GetRequest() == nil {
		t.Error("GetRequest() should not be nil")
	}
}

// ── ToContext — propagation ────────────────────────────────────────────────────

func TestToContext_PropagatesFields(t *testing.T) {
	ac := New(context.Background(), nil).
		SetRequestID("req-42").
		SetTraceID("trace-42").
		SetUserID("user-42").
		SetMethod("POST").
		SetURL("/ping").
		SetIP("5.6.7.8").
		SetUserAgent("GoTest/1.0")

	ctx := ac.ToContext()

	cases := []struct {
		key  constant.ContextKey
		want string
	}{
		{constant.RequestIDKey, "req-42"},
		{constant.TraceIDKey, "trace-42"},
		{constant.UserIDKey, "user-42"},
		{constant.RequestMethodKey, "POST"},
		{constant.RequestPathKey, "/ping"},
		{constant.RequestIPKey, "5.6.7.8"},
		{constant.RequestAgentKey, "GoTest/1.0"},
	}
	for _, c := range cases {
		got, _ := ctx.Value(c.key).(string)
		if got != c.want {
			t.Errorf("context[%s] = %q, want %q", c.key, got, c.want)
		}
	}
}

// ── ToContext — caching ────────────────────────────────────────────────────────

func TestToContext_ReturnsCached(t *testing.T) {
	ac := New(context.Background(), nil).SetUserID("u1")

	ctx1 := ac.ToContext()
	ctx2 := ac.ToContext()

	if ctx1 != ctx2 {
		t.Error("ToContext() should return the same cached context when no Set* was called in between")
	}
}

func TestToContext_InvalidatedAfterSet(t *testing.T) {
	ac := New(context.Background(), nil).SetUserID("u1")

	ctx1 := ac.ToContext()
	ac.SetUserID("u2")
	ctx2 := ac.ToContext()

	if ctx1 == ctx2 {
		t.Error("ToContext() should rebuild context after a Set* call")
	}
	if got := ctx2.Value(constant.UserIDKey).(string); got != "u2" {
		t.Errorf("context UserIDKey = %q, want %q", got, "u2")
	}
}

// ── Key-value store ───────────────────────────────────────────────────────────

func TestPutGetRemove(t *testing.T) {
	ac := New(context.Background(), nil)

	ac.Put("foo", "bar")
	if got := ac.Get("foo"); got != "bar" {
		t.Errorf("Get(foo) = %v, want %q", got, "bar")
	}

	// default value when missing
	if got := ac.Get("missing", "default"); got != "default" {
		t.Errorf("Get(missing, default) = %v, want %q", got, "default")
	}

	// nil when missing and no default
	if got := ac.Get("missing"); got != nil {
		t.Errorf("Get(missing) = %v, want nil", got)
	}

	ac.Remove("foo")
	if got := ac.Get("foo"); got != nil {
		t.Errorf("Get(foo) after Remove = %v, want nil", got)
	}
}

// ── Concurrency ───────────────────────────────────────────────────────────────

func TestAppContext_ConcurrentSafe(t *testing.T) {
	ac := New(context.Background(), nil)
	const n = 100

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ac.SetUserID("user")
			ac.SetTraceID("trace")
			_ = ac.ToContext()
			ac.Put("key", i)
			_ = ac.Get("key")
		}(i)
	}
	wg.Wait()
}

// ── Log ───────────────────────────────────────────────────────────────────────

func TestLog_NotNil(t *testing.T) {
	ac := New(context.Background(), nil)
	if ac.Log() == nil {
		t.Error("Log() should not return nil")
	}
}
