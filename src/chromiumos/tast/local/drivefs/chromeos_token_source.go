// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
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
func (tsf *ChromeOSTokenSourceFactory) TokenSource(ctx context.Context) oauth2.TokenSource {
	return &chromeOSTokenSource{
		ctx:    ctx,
		tconn:  tsf.tconn,
		email:  tsf.email,
		scopes: tsf.scopes,
	}
}

type chromeOSTokenSource struct {
	ctx    context.Context
	tconn  *chrome.TestConn
	email  string
	scopes []string
}

func (ts *chromeOSTokenSource) Token() (*oauth2.Token, error) {
	var accessToken string
	err := ts.tconn.Call(ts.ctx, &accessToken,
		"tast.promisify(chrome.autotestPrivate.getAccessToken)",
		ts.email, ts.scopes)
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken: accessToken,
		Expiry:      time.Now().Add(time.Minute * 45),
	}, nil
}
