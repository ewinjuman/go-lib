package httpstd

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponse_SaveToFile(t *testing.T) {
	t.Run("writes body to file", func(t *testing.T) {
		content := []byte("hello file content")
		resp := &Response{
			StatusCode:   200,
			Body:         content,
			SuccessCodes: []int{200},
		}
		path := filepath.Join(t.TempDir(), "output.txt")
		err := resp.SaveToFile(path)
		require.NoError(t, err)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("returns statusError when status not successful", func(t *testing.T) {
		resp := &Response{
			StatusCode:   404,
			Body:         []byte("not found"),
			SuccessCodes: []int{200},
		}
		err := resp.SaveToFile(filepath.Join(t.TempDir(), "out.txt"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "404")
	})

	t.Run("returns ErrEmptyResponseBody when body is nil", func(t *testing.T) {
		resp := &Response{
			StatusCode:   200,
			Body:         nil,
			SuccessCodes: []int{200},
		}
		err := resp.SaveToFile(filepath.Join(t.TempDir(), "out.txt"))
		assert.ErrorIs(t, err, ErrEmptyResponseBody)
	})

	t.Run("returns r.Error when request failed", func(t *testing.T) {
		sentinelErr := errors.New("connection refused")
		resp := &Response{
			Error: sentinelErr,
		}
		err := resp.SaveToFile(filepath.Join(t.TempDir(), "out.txt"))
		assert.ErrorIs(t, err, sentinelErr)
	})

	t.Run("propagates WriteFile error for invalid path", func(t *testing.T) {
		resp := &Response{
			StatusCode:   200,
			Body:         []byte("data"),
			SuccessCodes: []int{200},
		}
		err := resp.SaveToFile(filepath.Join(t.TempDir(), "missing_dir", "out.txt"))
		assert.Error(t, err)
	})
}
