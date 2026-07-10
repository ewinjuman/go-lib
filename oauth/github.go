package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	Error "github.com/ewinjuman/go-lib/v2/apperror"
	"golang.org/x/oauth2"
	xgithub "golang.org/x/oauth2/github"
)

// defaultGitHubHTTPTimeout bounds every outbound call to GitHub's REST API so
// a hung connection can't block a request indefinitely when the caller's
// context carries no deadline of its own.
const defaultGitHubHTTPTimeout = 10 * time.Second

type githubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

type githubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

type githubProvider struct {
	oauth2Config *oauth2.Config
	apiBaseURL   string       // overridable in tests; defaults to https://api.github.com
	httpClient   *http.Client // bounded timeout; never http.DefaultClient (unbounded)
}

// NewGitHubProvider builds a GitHub OAuth provider. GitHub is not
// OIDC-compliant (no id_token, no discovery document), so user info is
// fetched via GitHub's REST API instead.
func NewGitHubProvider(clientID, clientSecret, redirectURL string) Provider {
	return &githubProvider{
		oauth2Config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint:     xgithub.Endpoint,
			Scopes:       []string{"read:user", "user:email"},
		},
		apiBaseURL: "https://api.github.com",
		httpClient: &http.Client{Timeout: defaultGitHubHTTPTimeout},
	}
}

func (p *githubProvider) Name() string          { return "github" }
func (p *githubProvider) SupportsIDToken() bool { return false }

func (p *githubProvider) AuthCodeURL(state string, _ *PKCE) string {
	return p.oauth2Config.AuthCodeURL(state)
}

func (p *githubProvider) Exchange(ctx context.Context, code string, _ *PKCE) (*oauth2.Token, error) {
	token, err := p.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, Error.BadRequest(fmt.Sprintf("oauth: github code exchange failed: %v", err)).WithCause(err)
	}
	return token, nil
}

func (p *githubProvider) VerifyIDToken(context.Context, string, string) (*ProviderUser, error) {
	return nil, Error.BadRequest("oauth: github does not support id_token verification")
}

func (p *githubProvider) FetchUserInfo(ctx context.Context, token *oauth2.Token) (*ProviderUser, error) {
	user, err := p.getUser(ctx, token)
	if err != nil {
		return nil, err
	}

	// /user/emails' primary entry is the only authoritative source for
	// whether an address is verified. GitHub's public /user.email field
	// carries no verification guarantee on its own — a caller (the
	// downstream OAuthUseCase) uses EmailVerified to decide whether to
	// auto-link this login onto an existing account by email, so we never
	// infer "verified" merely from the field being non-empty. The public
	// field is used only as a last-resort fallback, and always as
	// unverified, when /user/emails can't be read at all.
	email, verified, primaryErr := p.getVerifiedPrimaryEmail(ctx, token)
	if primaryErr != nil {
		if user.Email == "" {
			return nil, primaryErr
		}
		email, verified = user.Email, false
	}

	return &ProviderUser{
		ProviderUserID: strconv.FormatInt(user.ID, 10),
		Email:          email,
		EmailVerified:  verified,
		Name:           user.Name,
		AvatarURL:      user.AvatarURL,
	}, nil
}

func (p *githubProvider) getUser(ctx context.Context, token *oauth2.Token) (*githubUser, error) {
	var user githubUser
	if err := p.getJSON(ctx, token, "/user", &user); err != nil {
		return nil, err
	}
	return &user, nil
}

// getVerifiedPrimaryEmail fetches /user/emails and returns the primary
// entry's address together with its authoritative "verified" flag. This is
// always the source of truth for EmailVerified; see FetchUserInfo.
func (p *githubProvider) getVerifiedPrimaryEmail(ctx context.Context, token *oauth2.Token) (email string, verified bool, err error) {
	var emails []githubEmail
	if err := p.getJSON(ctx, token, "/user/emails", &emails); err != nil {
		return "", false, err
	}
	for _, e := range emails {
		if e.Primary {
			return e.Email, e.Verified, nil
		}
	}
	return "", false, Error.BadRequest("oauth: github account has no primary email")
}

func (p *githubProvider) getJSON(ctx context.Context, token *oauth2.Token, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.apiBaseURL+path, nil)
	if err != nil {
		return Error.InternalError(fmt.Sprintf("oauth: github request to %s could not be built: %v", path, err)).WithCause(err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return Error.ServiceUnavailable(fmt.Sprintf("oauth: github request to %s failed: %v", path, err)).WithCause(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Error.Unauthorized(fmt.Sprintf("oauth: github request to %s returned %d", path, resp.StatusCode))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return Error.InternalError(fmt.Sprintf("oauth: github response from %s could not be decoded: %v", path, err)).WithCause(err)
	}
	return nil
}
