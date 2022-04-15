// Copyright 2022 The Chromium OS Authors. All rights reserved.
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

// ChromeOSTokenSourceFactory builds oauth2.TokenSource implementations that
// communicate with Chrome OS to generate OAuth tokens.
type ChromeOSTokenSourceFactory struct {
	tconn  *chrome.TestConn
	email  string
	scopes []string
}

// NewChromeOSTokenSourceFactory creates a new ChromeOSTokenSourceFactory
func NewChromeOSTokenSourceFactory(tconn *chrome.TestConn,
	email string, scopes []string) *ChromeOSTokenSourceFactory {
	return &ChromeOSTokenSourceFactory{
		tconn:  tconn,
		email:  email,
		scopes: scopes,
	}
}

// TokenSource builds a new oauth2.TokenSource that communicates with Chrome OS
// to generate OAuth tokens.
func (tsf *ChromeOSTokenSourceFactory) TokenSource(
	ctx context.Context) oauth2.TokenSource {
	return oauth2.ReuseTokenSource(nil, &chromeOSTokenSource{
		ctx:    ctx,
		tconn:  tsf.tconn,
		email:  tsf.email,
		scopes: tsf.scopes,
	})
}

type chromeOSTokenSource struct {
	ctx    context.Context
	tconn  *chrome.TestConn
	email  string
	scopes []string
}

func (ts *chromeOSTokenSource) Token() (*oauth2.Token, error) {
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiryMSec  string `json:"expiration_time_unix_ms"`
	}
	err := ts.tconn.Call(ts.ctx, &result,
		"tast.promisify(chrome.autotestPrivate.getAccessToken)",
		ts.email, ts.scopes)
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
