package auth

import (
	"context"
	"fmt"

	"github.com/watzon/alyx/internal/config"
)

type genericOIDCProvider struct {
	baseProvider
}

func newGenericOIDCProvider(name string, cfg config.OAuthProviderConfig) *genericOIDCProvider {
	return &genericOIDCProvider{
		baseProvider: baseProvider{
			name:         name,
			clientID:     cfg.ClientID,
			clientSecret: cfg.ClientSecret,
			scopes:       cfg.Scopes,
			authURL:      cfg.AuthURL,
			tokenURL:     cfg.TokenURL,
			userInfoURL:  cfg.UserInfoURL,
		},
	}
}

func (p *genericOIDCProvider) GetUserInfo(ctx context.Context, token *OAuthToken) (*OAuthUserInfo, error) {
	if p.userInfoURL == "" {
		return nil, fmt.Errorf("user info URL not configured for provider %s", p.name)
	}

	userData, err := fetchUserInfo(ctx, p.userInfoURL, token.AccessToken)
	if err != nil {
		return nil, err
	}

	userInfo := &OAuthUserInfo{
		Provider: p.name,
		RawData:  userData,
	}

	if id, ok := userData["sub"].(string); ok {
		userInfo.ID = id
	} else if id, ok := userData["id"].(string); ok {
		userInfo.ID = id
	}

	if email, ok := userData["email"].(string); ok {
		userInfo.Email = email
	}
	if verified, ok := userData["email_verified"].(bool); ok {
		userInfo.EmailVerified = verified
	}
	if name, ok := userData["name"].(string); ok {
		userInfo.Name = name
	}
	if picture, ok := userData["picture"].(string); ok {
		userInfo.AvatarURL = picture
	}

	if userInfo.Email == "" {
		return nil, ErrEmailRequired
	}

	return userInfo, nil
}
