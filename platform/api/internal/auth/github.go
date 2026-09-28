package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type githubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

type githubClient struct {
	cfg  Config
	http *http.Client
}

// authorizeURL requests identity only: no scopes, so no repository access (web-v1.md §11).
func (g githubClient) authorizeURL(state, verifier string) string {
	q := url.Values{
		"client_id":             {g.cfg.ClientID},
		"redirect_uri":          {g.cfg.CallbackURL()},
		"state":                 {state},
		"code_challenge":        {pkceChallenge(verifier)},
		"code_challenge_method": {"S256"},
		"allow_signup":          {"false"},
	}
	return g.cfg.AuthorizeURL + "?" + q.Encode()
}

// exchange trades the authorization code for an access token. The token is used once to read
// the user's identity and is never stored or logged.
func (g githubClient) exchange(ctx context.Context, code, verifier string) (string, error) {
	form := url.Values{
		"client_id":     {g.cfg.ClientID},
		"client_secret": {g.cfg.ClientSecret},
		"code":          {code},
		"redirect_uri":  {g.cfg.CallbackURL()},
		"code_verifier": {verifier},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	var body struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := g.doJSON(req, &body); err != nil {
		return "", fmt.Errorf("token exchange: %w", err)
	}
	// GitHub reports OAuth errors with HTTP 200 and an "error" field.
	if body.Error != "" {
		return "", fmt.Errorf("token exchange rejected: %s", body.Error)
	}
	if body.AccessToken == "" {
		return "", errors.New("token exchange returned no access token")
	}
	return body.AccessToken, nil
}

func (g githubClient) user(ctx context.Context, token string) (githubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.cfg.UserURL, nil)
	if err != nil {
		return githubUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	var u githubUser
	if err := g.doJSON(req, &u); err != nil {
		return githubUser{}, fmt.Errorf("read user: %w", err)
	}
	if u.ID <= 0 || u.Login == "" {
		return githubUser{}, errors.New("read user: missing id or login")
	}
	return u, nil
}

func (g githubClient) doJSON(req *http.Request, out any) error {
	req.Header.Set("User-Agent", "BrambiLab")
	resp, err := g.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}
