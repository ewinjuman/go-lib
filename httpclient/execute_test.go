// httpclient/execute_test.go
package httpclient

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newJSONServer creates an httptest.Server that always replies with status + JSON body.
func newJSONServer(t *testing.T, status int, body any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

// ─── buildBody ───────────────────────────────────────────────────────────────

func TestBuildBody_none_returnsNilReader(t *testing.T) {
	rb := New().Get("/")
	reader, ct, err := rb.buildBody()
	require.NoError(t, err)
	assert.Nil(t, reader)
	assert.Empty(t, ct)
}

func TestBuildBody_json_encodesBodyAsJSON(t *testing.T) {
	rb := New().Post("/").WithBody(map[string]string{"k": "v"})
	reader, ct, err := rb.buildBody()
	require.NoError(t, err)
	assert.Empty(t, ct) // Content-Type set via header separately
	b, _ := io.ReadAll(reader)
	var got map[string]string
	require.NoError(t, json.Unmarshal(b, &got))
	assert.Equal(t, "v", got["k"])
}

func TestBuildBody_form_encodesURLEncoded(t *testing.T) {
	rb := New().Post("/").WithForm(map[string]string{"user": "alice"})
	reader, ct, err := rb.buildBody()
	require.NoError(t, err)
	assert.Equal(t, "application/x-www-form-urlencoded", ct)
	b, _ := io.ReadAll(reader)
	assert.Contains(t, string(b), "user=alice")
}

func TestBuildBody_raw_returnsContentType(t *testing.T) {
	rb := New().Post("/").WithRawBody([]byte("<xml/>"), "application/xml")
	reader, ct, err := rb.buildBody()
	require.NoError(t, err)
	assert.Equal(t, "application/xml", ct)
	b, _ := io.ReadAll(reader)
	assert.Equal(t, "<xml/>", string(b))
}

func TestBuildBody_multipart_containsFieldAndFile(t *testing.T) {
	rb := New().Post("/").WithMultipart(
		Field("name", "avatar"),
		FileFromReader("file", "photo.jpg", strings.NewReader("IMGDATA")),
	)
	reader, ct, err := rb.buildBody()
	require.NoError(t, err)
	assert.Contains(t, ct, "multipart/form-data")
	b, _ := io.ReadAll(reader)
	s := string(b)
	assert.Contains(t, s, "avatar")
	assert.Contains(t, s, "IMGDATA")
}

// ─── buildURL ────────────────────────────────────────────────────────────────

func TestBuildURL_absoluteURL_noBaseURL(t *testing.T) {
	rb := New().Get("https://api.example.com/users")
	assert.Equal(t, "https://api.example.com/users", rb.buildURL())
}

func TestBuildURL_relativeURL_prependsBaseURL(t *testing.T) {
	rb := New(WithBaseURL("https://api.example.com")).Get("/users")
	assert.Equal(t, "https://api.example.com/users", rb.buildURL())
}

func TestBuildURL_pathParam_replaced(t *testing.T) {
	// WithPathParam takes map[string]string — NOT individual key+val strings
	rb := New(WithBaseURL("https://api.example.com")).Get("/users/:id").
		WithPathParam(map[string]string{"id": "42"})
	assert.Equal(t, "https://api.example.com/users/42", rb.buildURL())
}

func TestBuildURL_multiplePathParams(t *testing.T) {
	// Pass both params in one call — each WithPathParam call replaces rb.pathParams
	rb := New().Get("https://api.example.com/orgs/:org/users/:id").
		WithPathParam(map[string]string{"org": "acme", "id": "99"})
	assert.Equal(t, "https://api.example.com/orgs/acme/users/99", rb.buildURL())
}

// ─── Execute() integration ───────────────────────────────────────────────────

func TestExecute_GET_success(t *testing.T) {
	srv := newJSONServer(t, 200, map[string]string{"status": "ok"})
	defer srv.Close()

	resp := New().Get(srv.URL + "/ping").Execute()

	require.NoError(t, resp.Error)
	assert.Equal(t, 200, resp.StatusCode)
	assert.NotEmpty(t, resp.Body)
	assert.True(t, resp.IsSuccess())
}

func TestExecute_POST_sendsJSONBodyAndContentTypeHeader(t *testing.T) {
	type payload struct{ Name string }
	var received payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	New().Post(srv.URL).WithBody(payload{"alice"}).Execute()
	assert.Equal(t, "alice", received.Name)
}

func TestExecute_responseHeaders_populated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "hello")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	resp := New().Get(srv.URL).Execute()
	assert.Equal(t, "hello", resp.Headers.Get("X-Custom"))
}

func TestExecute_withOutput_streamsBodyAndBodyIsNil(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("stream data"))
	}))
	defer srv.Close()

	var buf strings.Builder
	resp := New().Get(srv.URL).WithOutput(&buf).Execute()
	require.NoError(t, resp.Error)
	assert.Nil(t, resp.Body)
	assert.Equal(t, "stream data", buf.String())
}

func TestExecute_nonSuccessStatus_isError(t *testing.T) {
	srv := newJSONServer(t, 404, map[string]string{"error": "not found"})
	defer srv.Close()

	resp := New().Get(srv.URL).Execute()
	assert.Equal(t, 404, resp.StatusCode)
	isErr, err := resp.IsError()
	assert.True(t, isErr)
	assert.Error(t, err)
}

func TestExecute_withBaseURL_relativePathJoined(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(200)
	}))
	defer srv.Close()

	New(WithBaseURL(srv.URL)).Get("/users/42").Execute()
	assert.Equal(t, "/users/42", gotPath)
}

func TestExecute_withQueryParams_sentInURL(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("search")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	New().Get(srv.URL).WithQueryParams(map[string]string{"search": "golang"}).Execute()
	assert.Equal(t, "golang", gotQuery)
}

func TestExecute_withContext_cancelledReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	resp := New().Get(srv.URL).WithContext(ctx).Execute()
	assert.Error(t, resp.Error)
}

func TestExecute_withCookie_sentToServer(t *testing.T) {
	var gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, _ := r.Cookie("session")
		if c != nil {
			gotCookie = c.Value
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	New().Get(srv.URL).WithCookie("session", "abc123").Execute()
	assert.Equal(t, "abc123", gotCookie)
}

func TestExecute_withDefaultBearer_setsAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	New(WithDefaultBearer(func() string { return "mytoken" })).Get(srv.URL).Execute()
	assert.Equal(t, "Bearer mytoken", gotAuth)
}
