// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package tape

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"

	"chromiumos/tast/errors"
)

// jwtToken stores the information needed to make a request to a JWT endpoint.
type jwtToken struct {
	conf     *oauth2.Config
	audience string
	refresh  *oauth2.Token
}

// jwtTokenResponse stores the information returned from the JWT token endpoint.
type jwtTokenResponse struct {
	IDToken string `json:"id_token"`
}

// Token will exchange a refresh token for a JWT that works with IAP (identity aware proxy).
// This was written because the oauth2 library does not support returning
// an id_token without a service account. This was adapted from:
// https://go-review.googlesource.com/c/build/+/361194.
func (j *jwtToken) Token() (*oauth2.Token, error) {
	v := url.Values{}
	v.Set("client_id", j.conf.ClientID)
	v.Set("client_secret", j.conf.ClientSecret)
	v.Set("refresh_token", j.refresh.RefreshToken)
	v.Set("grant_type", "refresh_token")
	v.Set("audience", j.audience)
	req, err := http.NewRequest("POST", j.conf.Endpoint.TokenURL, strings.NewReader(v.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("IAP token exchange failed: status %v, body %q", resp.Status, body)
	}

	var token jwtTokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, err
	}

	return &oauth2.Token{
		TokenType:   "Bearer",
		AccessToken: token.IDToken,
	}, nil
}
