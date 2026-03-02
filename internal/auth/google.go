package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleUser struct {
	Sub     string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

type GoogleAuth struct {
	oauthConf  *oauth2.Config
	adminEmail string
}

func NewGoogleAuth(clientID, secret, callbackURL, adminEmail string) *GoogleAuth {
	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: secret,
		RedirectURL:  callbackURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
	return &GoogleAuth{oauthConf: conf, adminEmail: adminEmail}
}

func (g *GoogleAuth) AuthURL(state string) string {
	return g.oauthConf.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (g *GoogleAuth) GetUser(ctx context.Context, code string) (*GoogleUser, error) {
	token, err := g.oauthConf.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("google: token exchange: %w", err)
	}

	client := g.oauthConf.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("google: userinfo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google: userinfo status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("google: read userinfo: %w", err)
	}

	var gu GoogleUser
	if err := json.Unmarshal(body, &gu); err != nil {
		return nil, fmt.Errorf("google: decode userinfo: %w", err)
	}
	return &gu, nil
}
