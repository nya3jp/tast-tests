// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	tokenRequestTimeout = 90 * time.Second
	tokenRequestURL     = "https://accounts.google.com"
)

type extensionTokenSource struct {
	ctx    context.Context
	chrome *chrome.Chrome
	tconn  *chrome.TestConn
	scopes []string
	email  string
}

// NewExtensionTokenSourceForAccount creates a new `oauth2.TokenSource` that
// uses the test extension to authorize requests for the given `email`.
func NewExtensionTokenSourceForAccount(
	ctx context.Context,
	chrome *chrome.Chrome, tconn *chrome.TestConn,
	scopes []string, email string) oauth2.TokenSource {
	return oauth2.ReuseTokenSource(nil, &extensionTokenSource{
		ctx:    ctx,
		chrome: chrome,
		tconn:  tconn,
		scopes: scopes,
		email:  email,
	})
}

// Token fetches a new OAuth token via the Chrome Identity API
func (ets *extensionTokenSource) Token() (*oauth2.Token, error) {
	ctx, cancel := context.WithTimeout(ets.ctx, tokenRequestTimeout)
	defer cancel()
	errChan := make(chan error)
	tokenChan := make(chan *oauth2.Token)
	// Launch the consent screen helper to click through the consent screen
	// if it pops up.
	go func() {
		if err := ets.maybeConsent(ctx); err != nil {
			errChan <- err
		}
	}()
	// Launch the auth flow, this may or may not open a consent screen.
	go func() {
		tok, err := ets.getAuthToken(ctx)
		if err != nil {
			errChan <- err
			return
		}
		tokenChan <- tok
	}()
	// Wait for a token or either of the above goroutines to fail.
	select {
	case token := <-tokenChan:
		return token, nil
	case err := <-errChan:
		return nil, err
	}

}

func (ets *extensionTokenSource) getAuthToken(
	ctx context.Context) (*oauth2.Token, error) {
	var token string
	var tokenRequest struct {
		Interactive bool     `json:"interactive"`
		Scopes      []string `json:"scopes"`
	}
	tokenRequest.Interactive = true
	tokenRequest.Scopes = ets.scopes
	err := ets.tconn.Call(ctx, &token,
		"tast.promisify(chrome.identity.getAuthToken)", tokenRequest)
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken: token,
	}, nil
}

// maybeConsent waits for the consent screen and clicks through it to approve
// the `getAuthToken` request.
func (ets *extensionTokenSource) maybeConsent(ctx context.Context) error {
	testing.ContextLog(ctx, "Waiting for consent page")
	extConn, err := ets.chrome.NewConnForTarget(
		ctx, chrome.MatchTargetURLPrefix(tokenRequestURL))
	if err != nil {
		return err
	}
	// Wait for the profile element to exist on the page.
	profileSelector := fmt.Sprintf(
		"document.querySelector('div[data-identifier=\"%s\"]')",
		ets.email)
	if err := extConn.WaitForExprFailOnErr(
		ctx, profileSelector); err != nil {
		return err
	}
	// Give the page a little more time to settle. Even after the page
	// completes loading, the profile element isn't ready to click.
	testing.Sleep(ctx, 5*time.Second)
	// Wait for and click the profile element on the oauth consent screen.
	if err := waitAndClickElement(ctx, extConn, profileSelector); err != nil {
		testing.ContextLog(ctx, "Failed to clieck profile element: ", err)
		return err
	}
	testing.ContextLog(ctx, "Found: ", ets.email)
	// Wait for and click the "Approve" button.
	if err := waitAndClickElement(ctx, extConn,
		"document.querySelector('#submit_approve_access')"); err != nil {
		testing.ContextLog(ctx, "Failed to approve access: ", err)
		return err
	}
	return nil
}
