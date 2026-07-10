package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func newTestFacebookAPIServer(t *testing.T, user facebookUser) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/me", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-access-token", r.URL.Query().Get("access_token"))
		assert.Equal(t, "id,name,email,picture", r.URL.Query().Get("fields"))
		_ = json.NewEncoder(w).Encode(user)
	})
	return httptest.NewServer(mux)
}

func TestFacebookProvider_FetchUserInfo_Success(t *testing.T) {
	srv := newTestFacebookAPIServer(t, facebookUser{ID: "fb-1", Name: "Face Book", Email: "face@example.com"})
	defer srv.Close()

	p := NewFacebookProvider("client-id", "client-secret", "https://app.example.com/callback").(*facebookProvider)
	p.graphBaseURL = srv.URL

	pu, err := p.FetchUserInfo(context.Background(), &oauth2.Token{AccessToken: "test-access-token"})
	require.NoError(t, err)
	assert.Equal(t, "fb-1", pu.ProviderUserID)
	assert.Equal(t, "face@example.com", pu.Email)
	// Facebook only ever returns a confirmed email address, so its presence
	// implies verified.
	assert.True(t, pu.EmailVerified)
	assert.Equal(t, "Face Book", pu.Name)
}

func TestFacebookProvider_FetchUserInfo_NoEmailGranted(t *testing.T) {
	srv := newTestFacebookAPIServer(t, facebookUser{ID: "fb-2", Name: "No Email"})
	defer srv.Close()

	p := NewFacebookProvider("client-id", "client-secret", "https://app.example.com/callback").(*facebookProvider)
	p.graphBaseURL = srv.URL

	pu, err := p.FetchUserInfo(context.Background(), &oauth2.Token{AccessToken: "test-access-token"})
	require.NoError(t, err)
	assert.Empty(t, pu.Email)
	assert.False(t, pu.EmailVerified)
}

func TestFacebookProvider_VerifyIDToken_NotSupported(t *testing.T) {
	p := NewFacebookProvider("client-id", "client-secret", "https://app.example.com/callback")
	assert.False(t, p.SupportsIDToken())
	_, err := p.VerifyIDToken(context.Background(), "irrelevant", "")
	assert.Error(t, err)
}

func TestFacebookProvider_HasTimeout(t *testing.T) {
	p := NewFacebookProvider("client-id", "client-secret", "https://app.example.com/callback").(*facebookProvider)
	assert.Greater(t, p.httpClient.Timeout, time.Duration(0))
}

func TestFacebookProvider_FetchUserInfo_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := NewFacebookProvider("client-id", "client-secret", "https://app.example.com/callback").(*facebookProvider)
	p.graphBaseURL = srv.URL

	_, err := p.FetchUserInfo(context.Background(), &oauth2.Token{AccessToken: "bad-token"})
	assert.Error(t, err)
}
