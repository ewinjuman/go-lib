package httpclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer membuat httptest.Server yang selalu reply status + body JSON tertentu.
func newTestServer(t *testing.T, status int, body interface{}) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			_ = json.NewEncoder(w).Encode(body)
		}
	}))
}

func TestRequest_DoRequest(t *testing.T) {
	t.Run("GET success — 200 dan body tidak kosong", func(t *testing.T) {
		srv := newTestServer(t, http.StatusOK, map[string]string{"status": "ok"})
		defer srv.Close()

		resp := Get(srv.URL + "/template").WithoutCircuitBreaker().Execute()

		assert.NoError(t, resp.Error)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotEmpty(t, resp.Body)
		assert.True(t, resp.IsSuccess())
	})

	t.Run("POST success — JSON body dikirim dan diterima server", func(t *testing.T) {
		type payload struct {
			Name string `json:"name"`
		}

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			var p payload
			require.NoError(t, json.NewDecoder(r.Body).Decode(&p))
			assert.Equal(t, "test", p.Name)

			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"created": p.Name})
		}))
		defer srv.Close()

		resp := Post(srv.URL).
			WithBody(payload{Name: "test"}).
			WithoutCircuitBreaker().
			Execute()

		assert.NoError(t, resp.Error)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("non-200 response — IsSuccess() false", func(t *testing.T) {
		srv := newTestServer(t, http.StatusNotFound, map[string]string{"error": "not found"})
		defer srv.Close()

		resp := Get(srv.URL).WithoutCircuitBreaker().Execute()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.False(t, resp.IsSuccess())
	})

	t.Run("custom success codes — 201 dianggap sukses", func(t *testing.T) {
		srv := newTestServer(t, http.StatusCreated, nil)
		defer srv.Close()

		resp := Get(srv.URL).
			WithSuccessCodes([]int{201}).
			WithoutCircuitBreaker().
			Execute()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.True(t, resp.IsSuccess())
	})

	t.Run("query params diteruskan ke server", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "bar", r.URL.Query().Get("foo"))
			assert.Equal(t, "2", r.URL.Query().Get("page"))
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		resp := Get(srv.URL).
			WithQueryParam(map[string]string{"foo": "bar", "page": "2"}).
			WithoutCircuitBreaker().
			Execute()

		assert.NoError(t, resp.Error)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("custom headers diteruskan ke server", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "myvalue", r.Header.Get("X-Custom-Header"))
			assert.Equal(t, "Bearer token123", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		resp := Get(srv.URL).
			WithHeaders(map[string]string{"X-Custom-Header": "myvalue"}).
			WithBearer("token123").
			WithoutCircuitBreaker().
			Execute()

		assert.NoError(t, resp.Error)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("connection error — resp.Error tidak nil, StatusCode 0", func(t *testing.T) {
		// Tutup server sebelum request untuk simulasi connection refused
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		url := srv.URL
		srv.Close()

		resp := Get(url).WithoutCircuitBreaker().Execute()

		assert.Error(t, resp.Error)
		assert.Equal(t, 0, resp.StatusCode)
		assert.False(t, resp.IsSuccess())
	})

	t.Run("Consume — unmarshal JSON ke struct", func(t *testing.T) {
		type apiResp struct {
			Status string `json:"status"`
			Code   int    `json:"code"`
		}

		srv := newTestServer(t, http.StatusOK, apiResp{Status: "ok", Code: 0})
		defer srv.Close()

		var result apiResp
		err := Get(srv.URL).
			WithoutCircuitBreaker().
			Execute().
			Consume(&result)

		require.NoError(t, err)
		assert.Equal(t, "ok", result.Status)
	})

	t.Run("path params disubstitusi di URL", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/users/42/orders/7", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		resp := Get(srv.URL+"/users/:userID/orders/:orderID").
			WithPathParam(map[string]string{"userID": "42", "orderID": "7"}).
			WithoutCircuitBreaker().
			Execute()

		assert.NoError(t, resp.Error)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("PUT request", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPut, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		resp := Put(srv.URL).
			WithBody(map[string]string{"key": "value"}).
			WithoutCircuitBreaker().
			Execute()

		assert.NoError(t, resp.Error)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("DELETE request — 204 No Content", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodDelete, r.Method)
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		resp := Delete(srv.URL).
			WithSuccessCodes([]int{204}).
			WithoutCircuitBreaker().
			Execute()

		assert.NoError(t, resp.Error)
		assert.True(t, resp.IsSuccess())
	})

	t.Run("PATCH request", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPatch, r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		resp := Patch(srv.URL).
			WithBody(map[string]string{"field": "updated"}).
			WithoutCircuitBreaker().
			Execute()

		assert.NoError(t, resp.Error)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("basic auth header dikirim", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, pass, ok := r.BasicAuth()
			assert.True(t, ok)
			assert.Equal(t, "admin", user)
			assert.Equal(t, "secret", pass)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		resp := Get(srv.URL).
			WithBasicAuth("admin", "secret").
			WithoutCircuitBreaker().
			Execute()

		assert.NoError(t, resp.Error)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}
