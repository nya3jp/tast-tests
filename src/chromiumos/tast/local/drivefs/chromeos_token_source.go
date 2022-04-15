// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"strconv"
	"time"

	"golang.org/x/oauth2"

	"chromiumos/tast/local/chrome"
)

type chromeOSTokenSource struct {
	ctx    context.Context
	tconn  *chrome.TestConn
	email  string
	scopes []string
}

// NewChromeOSTokenSourceForAccount creates a new `oauth2.TokenSource` that
// uses a private ChromeOS API to authorize requests for the given `email`.
//
// Note: This API is only available on test images and the command line flag
// `--get-access-token-for-test` must be specified to use it.
func NewChromeOSTokenSourceForAccount(ctx context.Context,
	tconn *chrome.TestConn, scopes []string, email string) oauth2.TokenSource {
	return &chromeOSTokenSource{
		ctx:    ctx,
		tconn:  tconn,
		email:  email,
		scopes: scopes,
	}
}

func (ts *chromeOSTokenSource) Token() (*oauth2.Token, error) {
	var params struct {
		Email     string   `json:"email"`
		Scopes    []string `json:"scopes"`
		TimeoutMs int64    `json:"timeoutMs,omitempty"`
	}
	var result struct {
		AccessToken string `json:"accessToken"`
		ExpiryMSec  string `json:"expirationTimeUnixMs"`
	}
	params.Email = ts.email
	params.Scopes = ts.scopes
	err := ts.tconn.Call(ts.ctx, &result,
		"tast.promisify(chrome.autotestPrivate.getAccessToken)", params)
	if err != nil {
		return nil, err
	}
	expiryMSec, err := strconv.ParseInt(result.ExpiryMSec, 10, 64)
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken: result.AccessToken,
		Expiry:      time.UnixMilli(expiryMSec),
	}, nil
}
