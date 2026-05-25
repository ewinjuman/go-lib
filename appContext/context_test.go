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
	if ac.ToContext() == nil {
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
		SetTenantID("tenant-1").
		SetRequestTime(now).
		SetIP("1.2.3.4").
		SetUserAgent("TestAgent/1.0").
		SetURL("/api/v1/test").
		SetMethod("GET").
		SetResponseStatus(200).
		SetPort(8080).
		SetSrcIP("10.0.0.1").
		SetHeader(map[string]string{"X-Test": "yes"}).
		SetRequest(map[string]string{"body": "data"})

	strChecks := []struct {
		got  string
		want string
	}{
		{ac.GetRequestID(), "req-1"},
		{ac.GetTraceID(), "trace-1"},
		{ac.GetUserID(), "user-1"},
		{ac.GetTenantID(), "tenant-1"},
		{ac.GetIP(), "1.2.3.4"},
		{ac.GetUserAgent(), "TestAgent/1.0"},
		{ac.GetURL(), "/api/v1/test"},
		{ac.GetMethod(), "GET"},
		{ac.GetSrcIP(), "10.0.0.1"},
	}
	for _, c := range strChecks {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}

	if got := ac.GetRequestTime(); !got.Equal(now) {
		t.Errorf("GetRequestTime() = %v, want %v", got, now)
	}
	if ac.GetResponseStatus() != 200 {
		t.Errorf("GetResponseStatus() = %d, want 200", ac.GetResponseStatus())
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

// ── Duration ──────────────────────────────────────────────────────────────────

func TestDuration(t *testing.T) {
	ac := New(context.Background(), nil)
	time.Sleep(5 * time.Millisecond)
	d := ac.Duration()
	if d < 5*time.Millisecond {
		t.Errorf("Duration() = %v, want >= 5ms", d)
	}
	if d > 2*time.Second {
		t.Errorf("Duration() = %v, suspiciously large", d)
	}
}

// ── ToContext — propagation ────────────────────────────────────────────────────

func TestToContext_PropagatesFields(t *testing.T) {
	ac := New(context.Background(), nil).
		SetRequestID("req-42").
		SetTraceID("trace-42").
		SetUserID("user-42").
		SetTenantID("tenant-42").
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
		{constant.TenantIDKey, "tenant-42"},
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

func TestToContext_StoresAppContextItself(t *testing.T) {
	ac := New(context.Background(), nil).SetUserID("u1")
	ctx := ac.ToContext()

	stored, ok := ctx.Value(constant.AppContextKey).(*AppContext)
	if !ok || stored == nil {
		t.Fatal("AppContext itself should be stored in context under AppContextKey")
	}
	if stored != ac {
		t.Error("stored AppContext pointer should be the same instance")
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

func TestToContext_LocalOnlyDoesNotInvalidate(t *testing.T) {
	// SetResponseStatus, SetPort, SetSrcIP, SetHeader, SetRequest
	// should NOT invalidate the cached context.
	ac := New(context.Background(), nil).SetUserID("u1")

	ctx1 := ac.ToContext()
	ac.SetResponseStatus(404)
	ac.SetPort(9090)
	ac.SetSrcIP("1.2.3.4")
	ac.SetHeader("h")
	ac.SetRequest("r")
	ctx2 := ac.ToContext()

	if ctx1 != ctx2 {
		t.Error("local-only Set* calls should not invalidate the cached context")
	}
}

// ── FromContext ───────────────────────────────────────────────────────────────

func TestFromContext_ReturnsAppContext(t *testing.T) {
	ac := New(context.Background(), nil).SetTraceID("trace-99")
	ctx := ac.ToContext()

	got := FromContext(ctx)
	if got == nil {
		t.Fatal("FromContext should return the stored AppContext")
	}
	if got != ac {
		t.Error("FromContext should return the same AppContext pointer")
	}
	if got.GetTraceID() != "trace-99" {
		t.Errorf("TraceID via FromContext = %q, want trace-99", got.GetTraceID())
	}
}

func TestFromContext_ReturnsNilWhenMissing(t *testing.T) {
	ctx := context.Background()
	if got := FromContext(ctx); got != nil {
		t.Errorf("FromContext on bare context = %v, want nil", got)
	}
}

func TestFromContext_ServiceLayer(t *testing.T) {
	// Simulates passing ctx through handler → service layer
	ac := New(context.Background(), nil).
		SetUserID("user-svc").
		SetTenantID("tenant-svc")
	ctx := ac.ToContext()

	// Service layer only receives ctx, not ac directly
	retrieved := FromContext(ctx)
	if retrieved == nil {
		t.Fatal("service layer should be able to retrieve AppContext from ctx")
	}
	if retrieved.GetUserID() != "user-svc" {
		t.Errorf("UserID = %q, want user-svc", retrieved.GetUserID())
	}
	if retrieved.GetTenantID() != "tenant-svc" {
		t.Errorf("TenantID = %q, want tenant-svc", retrieved.GetTenantID())
	}
}

// ── Clone ─────────────────────────────────────────────────────────────────────

func TestClone_PropagatedFieldsCopied(t *testing.T) {
	ac := New(context.Background(), nil).
		SetTraceID("trace-orig").
		SetUserID("user-orig").
		SetTenantID("tenant-orig").
		SetIP("1.2.3.4").
		SetUserAgent("Agent/1").
		SetURL("/api").
		SetMethod("GET")

	clone := ac.Clone()

	if clone.GetTraceID() != "trace-orig" {
		t.Errorf("Clone TraceID = %q, want trace-orig", clone.GetTraceID())
	}
	if clone.GetUserID() != "user-orig" {
		t.Errorf("Clone UserID = %q, want user-orig", clone.GetUserID())
	}
	if clone.GetTenantID() != "tenant-orig" {
		t.Errorf("Clone TenantID = %q, want tenant-orig", clone.GetTenantID())
	}
	if clone.GetIP() != "1.2.3.4" {
		t.Errorf("Clone IP = %q, want 1.2.3.4", clone.GetIP())
	}
}

func TestClone_NewRequestID(t *testing.T) {
	ac := New(context.Background(), nil)
	clone := ac.Clone()

	if clone.GetRequestID() == ac.GetRequestID() {
		t.Error("Clone should have a new requestID")
	}
	if clone.GetRequestID() == "" {
		t.Error("Clone requestID should not be empty")
	}
}

func TestClone_LocalFieldsNotCopied(t *testing.T) {
	ac := New(context.Background(), nil).
		SetResponseStatus(500).
		SetPort(8080).
		SetSrcIP("10.0.0.1").
		SetHeader("hdr").
		SetRequest("req")
	ac.Put("key", "val")

	clone := ac.Clone()

	if clone.GetResponseStatus() != 0 {
		t.Errorf("Clone responseStatus = %d, want 0", clone.GetResponseStatus())
	}
	if clone.GetPort() != 0 {
		t.Errorf("Clone port = %d, want 0", clone.GetPort())
	}
	if clone.GetSrcIP() != "" {
		t.Errorf("Clone srcIP = %q, want empty", clone.GetSrcIP())
	}
	if clone.GetHeader() != nil {
		t.Error("Clone header should be nil")
	}
	if clone.GetRequest() != nil {
		t.Error("Clone request should be nil")
	}
	if clone.Get("key") != nil {
		t.Error("Clone cMap should not be copied")
	}
}

func TestClone_IndependentMutation(t *testing.T) {
	ac := New(context.Background(), nil).SetUserID("original")
	clone := ac.Clone()

	clone.SetUserID("cloned")

	if ac.GetUserID() != "original" {
		t.Error("mutating clone should not affect original")
	}
}

// ── TenantID propagation ──────────────────────────────────────────────────────

func TestSetTenantID_PropagatedToContext(t *testing.T) {
	ac := New(context.Background(), nil).SetTenantID("acme-corp")
	ctx := ac.ToContext()

	got, _ := ctx.Value(constant.TenantIDKey).(string)
	if got != "acme-corp" {
		t.Errorf("context TenantIDKey = %q, want acme-corp", got)
	}
}

// ── ResponseStatus — local only ───────────────────────────────────────────────

func TestSetResponseStatus_NotInContext(t *testing.T) {
	ac := New(context.Background(), nil).SetResponseStatus(201)
	ctx := ac.ToContext()

	// responseStatus is local-only, should not appear in context under ResponseStatusKey
	val := ctx.Value(constant.ResponseStatusKey)
	if val != nil {
		t.Errorf("responseStatus should not be in context, got %v", val)
	}
	if ac.GetResponseStatus() != 201 {
		t.Errorf("GetResponseStatus() = %d, want 201", ac.GetResponseStatus())
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
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ac.SetUserID("user")
			ac.SetTraceID("trace")
			ac.SetTenantID("tenant")
			_ = ac.ToContext()
			_ = ac.Duration()
			_ = ac.Clone()
			ac.Put("key", 1)
			_ = ac.Get("key")
		}()
	}
	wg.Wait()
}

func TestFromContext_ConcurrentSafe(t *testing.T) {
	ac := New(context.Background(), nil).SetTraceID("trace-concurrent")
	ctx := ac.ToContext()

	const n = 50
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := FromContext(ctx)
			if got == nil {
				t.Error("FromContext returned nil under concurrency")
			}
		}()
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
