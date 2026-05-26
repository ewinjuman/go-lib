// httpclient/response_test.go
package httpclient

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jsonBody(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestResponse_IsSuccess(t *testing.T) {
	assert.True(t, (&Response{StatusCode: 200, SuccessCodes: []int{200}}).IsSuccess())
	assert.False(t, (&Response{StatusCode: 404, SuccessCodes: []int{200}}).IsSuccess())
	assert.False(t, (&Response{StatusCode: 200, Error: errors.New("e"), SuccessCodes: []int{200}}).IsSuccess())
}

func TestResponse_IsError(t *testing.T) {
	isErr, err := (&Response{StatusCode: 200, SuccessCodes: []int{200}}).IsError()
	assert.False(t, isErr)
	assert.NoError(t, err)

	isErr, err = (&Response{StatusCode: 500, SuccessCodes: []int{200}}).IsError()
	assert.True(t, isErr)
	assert.Error(t, err)
}

func TestResponse_Consume_success(t *testing.T) {
	type result struct{ Name string }
	r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: jsonBody(result{"alice"})}
	var got result
	require.NoError(t, r.Consume(&got))
	assert.Equal(t, "alice", got.Name)
}

func TestResponse_Consume_errorPropagated(t *testing.T) {
	r := &Response{Error: errors.New("network error"), SuccessCodes: []int{200}}
	assert.Error(t, r.Consume(nil))
}

func TestResponse_Consume_nonSuccessStatus(t *testing.T) {
	r := &Response{StatusCode: 404, SuccessCodes: []int{200}, Body: []byte(`{"error":"not found"}`)}
	assert.Error(t, r.Consume(nil))
}

func TestResponse_Consume_nilBody_returnsErrEmpty(t *testing.T) {
	r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: nil}
	assert.ErrorIs(t, r.Consume(nil), ErrEmptyResponseBody)
}

func TestResponse_ConsumeXML(t *testing.T) {
	type root struct {
		Value string `xml:"value"`
	}
	xmlBytes := []byte(`<root><value>hello</value></root>`)
	r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: xmlBytes}
	var got root
	require.NoError(t, r.ConsumeXML(&got))
	assert.Equal(t, "hello", got.Value)
}

func TestResponse_ConsumeText(t *testing.T) {
	r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: []byte("plain text")}
	text, err := r.ConsumeText()
	require.NoError(t, err)
	assert.Equal(t, "plain text", text)
}

func TestResponse_SaveToFile(t *testing.T) {
	t.Run("writes body to file", func(t *testing.T) {
		r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: []byte("file content")}
		path := filepath.Join(t.TempDir(), "out.txt")
		require.NoError(t, r.SaveToFile(path))
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "file content", string(got))
	})

	t.Run("returns statusError when status not successful", func(t *testing.T) {
		r := &Response{StatusCode: 404, SuccessCodes: []int{200}, Body: []byte("not found")}
		err := r.SaveToFile(filepath.Join(t.TempDir(), "out.txt"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("returns ErrEmptyResponseBody when body is nil", func(t *testing.T) {
		r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: nil}
		err := r.SaveToFile(filepath.Join(t.TempDir(), "out.txt"))
		assert.ErrorIs(t, err, ErrEmptyResponseBody)
	})

	t.Run("returns r.Error when request failed", func(t *testing.T) {
		sentinelErr := errors.New("connection refused")
		r := &Response{Error: sentinelErr}
		err := r.SaveToFile(filepath.Join(t.TempDir(), "out.txt"))
		assert.ErrorIs(t, err, sentinelErr)
	})

	t.Run("propagates WriteFile error for invalid path", func(t *testing.T) {
		r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: []byte("data")}
		err := r.SaveToFile(filepath.Join(t.TempDir(), "missing_dir", "out.txt"))
		assert.Error(t, err)
	})
}

func TestResponse_Headers_accessible(t *testing.T) {
	h := http.Header{"X-Custom": []string{"value"}}
	r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Headers: h}
	assert.Equal(t, "value", r.Headers.Get("X-Custom"))
}

func TestResponse_Raw_returnsUnderlyingResponse(t *testing.T) {
	raw := &http.Response{StatusCode: 200}
	r := &Response{raw: raw}
	assert.Same(t, raw, r.Raw())
}

func TestResponse_HttpCode(t *testing.T) {
	r := &Response{StatusCode: 201}
	assert.Equal(t, 201, r.HttpCode())
}

func TestResponse_ConsumeXML_nilBody_returnsErrEmpty(t *testing.T) {
	r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: nil}
	assert.ErrorIs(t, r.ConsumeXML(nil), ErrEmptyResponseBody)
}

func TestResponse_ConsumeText_nilBody_returnsErrEmpty(t *testing.T) {
	r := &Response{StatusCode: 200, SuccessCodes: []int{200}, Body: nil}
	_, err := r.ConsumeText()
	assert.ErrorIs(t, err, ErrEmptyResponseBody)
}
