package oauth

import (
	"context"
	"fmt"

	"github.com/coreos/go-oidc/v3/oidc"
	Error "github.com/ewinjuman/go-lib/v2/apperror"
	"golang.org/x/oauth2"
)

// OIDCConfig configures a generic OpenID Connect provider via discovery.
// Any OIDC-compliant issuer (Google, Microsoft, Okta, ...) can be
// registered this way without a bespoke Provider implementation.
type OIDCConfig struct {
	Name         string
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	// Scopes are appended to the default "openid email profile".
	Scopes []string
}

type oidcProvider struct {
	name         string
	verifier     *oidc.IDTokenVerifier
	oauth2Config *oauth2.Config
}

// NewOIDCProvider discovers cfg.IssuerURL's OIDC configuration and builds a
// Provider backed by it.
func NewOIDCProvider(ctx context.Context, cfg OIDCConfig) (Provider, error) {
	p, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		return nil, Error.ServiceUnavailable(fmt.Sprintf("oauth: %s discovery failed: %v", cfg.Name, err)).WithCause(err)
	}

	scopes := append([]string{oidc.ScopeOpenID, "email", "profile"}, cfg.Scopes...)
	return &oidcProvider{
		name:     cfg.Name,
		verifier: p.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
		oauth2Config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     p.Endpoint(),
			Scopes:       scopes,
		},
	}, nil
}

// NewGoogleProvider is a convenience wrapper around NewOIDCProvider
// pre-configured with Google's issuer URL, since Google is itself a
// standard OIDC provider.
func NewGoogleProvider(ctx context.Context, clientID, clientSecret, redirectURL string) (Provider, error) {
	return NewOIDCProvider(ctx, OIDCConfig{
		Name:         "google",
		IssuerURL:    "https://accounts.google.com",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
	})
}

func (p *oidcProvider) Name() string          { return p.name }
func (p *oidcProvider) SupportsIDToken() bool { return true }

func (p *oidcProvider) AuthCodeURL(state string, pkce *PKCE) string {
	opts := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline}
	if pkce != nil {
		opts = append(opts, oauth2.S256ChallengeOption(pkce.Verifier))
	}
	return p.oauth2Config.AuthCodeURL(state, opts...)
}

func (p *oidcProvider) Exchange(ctx context.Context, code string, pkce *PKCE) (*oauth2.Token, error) {
	var opts []oauth2.AuthCodeOption
	if pkce != nil {
		opts = append(opts, oauth2.VerifierOption(pkce.Verifier))
	}
	token, err := p.oauth2Config.Exchange(ctx, code, opts...)
	if err != nil {
		return nil, Error.BadRequest(fmt.Sprintf("oauth: %s code exchange failed: %v", p.name, err)).WithCause(err)
	}
	return token, nil
}

func (p *oidcProvider) FetchUserInfo(ctx context.Context, token *oauth2.Token) (*ProviderUser, error) {
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, Error.BadRequest(fmt.Sprintf("oauth: %s token response missing id_token", p.name))
	}
	return p.VerifyIDToken(ctx, rawIDToken, "")
}

func (p *oidcProvider) VerifyIDToken(ctx context.Context, rawIDToken, nonce string) (*ProviderUser, error) {
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, Error.Unauthorized(fmt.Sprintf("oauth: %s id_token verification failed: %v", p.name, err)).WithCause(err)
	}
	if nonce != "" && idToken.Nonce != nonce {
		return nil, Error.Unauthorized(fmt.Sprintf("oauth: %s id_token nonce mismatch", p.name))
	}

	var claims struct {
		Subject       string `json:"sub"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, Error.Unauthorized(fmt.Sprintf("oauth: %s id_token claims decode failed: %v", p.name, err)).WithCause(err)
	}

	return &ProviderUser{
		ProviderUserID: claims.Subject,
		Email:          claims.Email,
		EmailVerified:  claims.EmailVerified,
		Name:           claims.Name,
		AvatarURL:      claims.Picture,
	}, nil
}
