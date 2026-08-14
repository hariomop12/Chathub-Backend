package auth

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
)

type Claims struct {
	Sub     string
	Email   string
	Name    string
	Picture string
}

type Verifier interface {
	Verify(ctx context.Context, raw string) (*Claims, error)
}

// GoogleVerifier verifies Google Sign-In ID tokens. It checks the signature
// against Google's JWKS, the issuer (accounts.google.com) and the audience
// (our OAuth client id). ID tokens are valid for ~1 hour, no sessions needed.
type GoogleVerifier struct {
	verifier *oidc.IDTokenVerifier
}

func NewGoogleVerifier(ctx context.Context, clientID string) (*GoogleVerifier, error) {
	provider, err := oidc.NewProvider(ctx, "https://accounts.google.com")
	if err != nil {
		return nil, err
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: clientID})
	return &GoogleVerifier{verifier: verifier}, nil
}

func (g *GoogleVerifier) Verify(ctx context.Context, raw string) (*Claims, error) {
	idToken, err := g.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, err
	}
	var claims struct {
		Sub     string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, err
	}
	return &Claims{
		Sub:     claims.Sub,
		Email:   claims.Email,
		Name:    claims.Name,
		Picture: claims.Picture,
	}, nil
}
