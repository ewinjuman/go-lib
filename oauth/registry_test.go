package oauth

import (
	"context"
	"testing"

	Error "github.com/ewinjuman/go-lib/v2/apperror"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
)

type stubProvider struct{ name string }

func (s *stubProvider) Name() string                     { return s.name }
func (s *stubProvider) SupportsIDToken() bool            { return false }
func (s *stubProvider) AuthCodeURL(string, *PKCE) string { return "" }
func (s *stubProvider) Exchange(context.Context, string, *PKCE) (*oauth2.Token, error) {
	return nil, nil
}
func (s *stubProvider) FetchUserInfo(context.Context, *oauth2.Token) (*ProviderUser, error) {
	return nil, nil
}
func (s *stubProvider) VerifyIDToken(context.Context, string, string) (*ProviderUser, error) {
	return nil, nil
}

func TestRegistry_Get_Found(t *testing.T) {
	r := NewRegistry(&stubProvider{name: "google"}, &stubProvider{name: "github"})
	p, err := r.Get("github")
	assert.NoError(t, err)
	assert.Equal(t, "github", p.Name())
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry(&stubProvider{name: "google"})
	_, err := r.Get("facebook")
	assert.Error(t, err)
	assert.Equal(t, 404, Error.GetCode(err))
}
