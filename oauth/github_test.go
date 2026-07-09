package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

func newTestGitHubAPIServer(t *testing.T, user githubUser, emails []githubEmail) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-access-token", r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(user)
	})
	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(emails)
	})
	return httptest.NewServer(mux)
}

func TestGitHubProvider_FetchUserInfo_PublicEmail(t *testing.T) {
	srv := newTestGitHubAPIServer(t, githubUser{ID: 1, Login: "octocat", Name: "Octo Cat", Email: "octo@example.com", AvatarURL: "https://x/avatar.png"}, nil)
	defer srv.Close()

	p := NewGitHubProvider("client-id", "client-secret", "https://app.example.com/callback").(*githubProvider)
	p.apiBaseURL = srv.URL

	pu, err := p.FetchUserInfo(context.Background(), &oauth2.Token{AccessToken: "test-access-token"})
	require.NoError(t, err)
	assert.Equal(t, "1", pu.ProviderUserID)
	assert.Equal(t, "octo@example.com", pu.Email)
	assert.True(t, pu.EmailVerified)
	assert.Equal(t, "Octo Cat", pu.Name)
}

func TestGitHubProvider_FetchUserInfo_PrivateEmail_FallsBackToEmailsEndpoint(t *testing.T) {
	srv := newTestGitHubAPIServer(t,
		githubUser{ID: 2, Login: "hidden", Name: "Hidden Email"},
		[]githubEmail{
			{Email: "secondary@example.com", Primary: false, Verified: true},
			{Email: "primary@example.com", Primary: true, Verified: true},
		},
	)
	defer srv.Close()

	p := NewGitHubProvider("client-id", "client-secret", "https://app.example.com/callback").(*githubProvider)
	p.apiBaseURL = srv.URL

	pu, err := p.FetchUserInfo(context.Background(), &oauth2.Token{AccessToken: "test-access-token"})
	require.NoError(t, err)
	assert.Equal(t, "primary@example.com", pu.Email)
	assert.True(t, pu.EmailVerified)
}

func TestGitHubProvider_VerifyIDToken_NotSupported(t *testing.T) {
	p := NewGitHubProvider("client-id", "client-secret", "https://app.example.com/callback")
	assert.False(t, p.SupportsIDToken())
	_, err := p.VerifyIDToken(context.Background(), "irrelevant", "")
	assert.Error(t, err)
}

func TestGitHubProvider_AuthCodeURL(t *testing.T) {
	p := NewGitHubProvider("client-id", "client-secret", "https://app.example.com/callback")
	url := p.AuthCodeURL("state-1", nil)
	assert.Contains(t, url, "github.com/login/oauth/authorize")
	assert.Contains(t, url, "state=state-1")
}
