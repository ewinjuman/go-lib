package bench_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	httpclient "github.com/ewinjuman/go-lib/v2/httpclient"
)

//run: go test -run=^$ -bench=. -benchmem -benchtime=3s ./bench/...

// testPayload is used for POST benchmarks.
type testPayload struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// newTestServer starts a local server for benchmarks to hit.
func newTestServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	})

	mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		var body testPayload
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		out, _ := json.Marshal(body)
		_, _ = w.Write(out)
	})

	return httptest.NewServer(mux)
}

// ──────────────────────────────────────────────
// GET benchmarks
// ──────────────────────────────────────────────

func BenchmarkHTTPClient_GET(b *testing.B) {
	srv := newTestServer()
	defer srv.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = httpclient.Get(srv.URL + "/get").Execute()
	}
}

// ──────────────────────────────────────────────
// POST benchmarks
// ──────────────────────────────────────────────

func BenchmarkHTTPClient_POST(b *testing.B) {
	srv := newTestServer()
	defer srv.Close()
	payload := testPayload{Name: "benchmark", Value: 42}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = httpclient.Post(srv.URL + "/post").WithBody(payload).Execute()
	}
}

// ──────────────────────────────────────────────
// Parallel benchmarks (concurrent requests)
// ──────────────────────────────────────────────

func BenchmarkHTTPClient_GET_Parallel(b *testing.B) {
	srv := newTestServer()
	defer srv.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = httpclient.Get(srv.URL + "/get").Execute()
		}
	})
}

func BenchmarkHTTPClient_POST_Parallel(b *testing.B) {
	srv := newTestServer()
	defer srv.Close()
	payload := testPayload{Name: "benchmark", Value: 42}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = httpclient.Post(srv.URL + "/post").WithBody(payload).Execute()
		}
	})
}
