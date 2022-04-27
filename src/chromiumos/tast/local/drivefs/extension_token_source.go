// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package drivefs

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/testing"
)

const (
	tokenRequestTimeout = 90 * time.Second
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
	err := ets.tconn.Call(ctx, &token, "tast.promisify(chrome.identity.getAuthToken)", tokenRequest)
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
	const (
		// The URL of the consent page.
		tokenRequestURL = "https://accounts.google.com"
	)
	var (
		// A selector template for the profile on the consent page.
		// Param 0 is the user's email.
		profileSelectorTmpl = buildXPathSelector("//div[@data-identifier=\"%s\"]")

		// A selector for the "Go to ... (unsafe)" link on the unsafe
		// app page.
		unsafeAppSelector = buildXPathSelector("//a[contains(text(),\"(unsafe)\")]")

		// A selector for the "Allow" button on the consent page
		allowSelector = buildXPathSelector("//span[text()=\"Allow\"]")
	)
	testing.ContextLog(ctx, "Maybe waiting for consent page")
	extConn, err := ets.chrome.NewConnForTarget(
		ctx, chrome.MatchTargetURLPrefix(tokenRequestURL))
	if err != nil {
		return err
	}
	// Wait for the profile element to exist on the page.
	profileSelector := fmt.Sprintf(profileSelectorTmpl, ets.email)
	if err := extConn.WaitForExprFailOnErr(
		ctx, profileSelector); err != nil {
		return err
	}
	// Give the page a little more time to settle. Even after the page
	// completes loading, the profile element isn't ready to click.
	testing.Sleep(ctx, 5*time.Second)
	// Wait for and click the profile element on the oauth consent screen.
	if err := waitAndClickElementChangesPage(ctx, extConn, profileSelector); err != nil {
		return errors.Wrap(err, "failed to click profile element")
	}
	testing.ContextLog(ctx, "Found consent page for: ", ets.email)
	// Wait for and click the unverified app continue button.
	if err := waitAndClickElementChangesPage(ctx, extConn, unsafeAppSelector); err != nil {
		return errors.Wrap(err, "failed to click 'Go to ... (unsafe)'")
	}
	// Wait for and click the "Allow" button.
	if err := waitAndClickElement(ctx, extConn, allowSelector); err != nil {
		return errors.Wrap(err, "failed to click 'Allow'")
	}
	testing.ContextLog(ctx, "Approved access for: ", ets.email)
	return nil
}

// waitAndClickElementChangesPage waits for an element, clicks it and then
// waits for the page to change and for it to load.
func waitAndClickElementChangesPage(ctx context.Context, conn *chrome.Conn, jsExpr string) error {
	startPath := ""
	if err := conn.Eval(ctx, "location.pathname", &startPath); err != nil {
		return errors.Wrap(err, "failed to get starting path")
	}
	if err := waitAndClickElement(ctx, conn, jsExpr); err != nil {
		return err
	}
	// We've clicked the element, wait for the page to change and complete
	// loading.
	locationChangeExpr := fmt.Sprintf("location.pathname !== '%s'", startPath)
	if err := conn.WaitForExprFailOnErrWithTimeout(ctx, locationChangeExpr, time.Minute); err != nil {
		return errors.Wrap(err, "failed to wait for the path to change")
	}
	const pageLoadExpr = "document.readyState === 'complete'"
	if err := conn.WaitForExprFailOnErrWithTimeout(ctx, pageLoadExpr, time.Minute); err != nil {
		return errors.Wrap(err, "failed to wait for Path to change")
	}
	return nil
}

func buildXPathSelector(xpath string) string {
	return fmt.Sprintf("document.evaluate('%s', document, null, XPathResult.FIRST_ORDERED_NODE_TYPE).singleNodeValue", xpath)
}
