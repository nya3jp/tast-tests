// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

// WebAuthnInWebAuthnIo performs the WebAuthn procedure in the external site webauthn.io.
func WebAuthnInWebAuthnIo(ctx context.Context, cr *chrome.Chrome, br *browser.Browser, authCallback func(context.Context, *uiauto.Context) error) error {
	// We need truly random values for username strings so that different test runs don't affect each other.
	rand.Seed(time.Now().UnixNano())

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test API connection")
	}

	conn, err := br.NewConn(ctx, "https://webauthn.io/")
	if err != nil {
		return errors.Wrap(err, "failed to navigate to test website")
	}
	defer func(ctx context.Context, conn *chrome.Conn) {
		conn.Navigate(ctx, "https://webauthn.io/logout")
		conn.CloseTarget(ctx)
		conn.Close()
	}(cleanupCtx, conn)

	// Perform MakeCredential on the test website.

	// Enter username
	name := randomUsername()
	testing.ContextLogf(ctx, "Username: %s", name)
	// Use a random username because webauthn.io keeps state for each username for a period of time.
	err = conn.Eval(ctx, fmt.Sprintf(`document.getElementById('input-email')._x_model.set("%s")`, name), nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression to set username")
	}

	// Press "Register" button.
	err = conn.Eval(ctx, `document.getElementById('register-button').click()`, nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression to press register button")
	}

	ui := uiauto.New(tconn)

	// Choose platform authenticator
	platformAuthenticatorButton := nodewith.Role(role.Button).Name("This device")
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(platformAuthenticatorButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to select platform authenticator from transport selection sheet")
	}
	if err = ui.DoDefault(platformAuthenticatorButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click button for platform authenticator")
	}

	// Wait for ChromeOS WebAuthn dialog.
	dialog := nodewith.ClassName("AuthDialogWidget")
	if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(dialog)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for the ChromeOS dialog")
	}

	if err := authCallback(ctx, ui); err != nil {
		return errors.Wrap(err, "failed to call authCallback")
	}

	if err := CheckMakeCredentialSuccessInWebAuthnIo(ctx, conn); err != nil {
		return errors.Wrap(err, "failed to perform MakeCredential")
	}

	// Perform GetAssertion on the test website.

	// Press "Login" button.
	err = conn.Eval(ctx, `document.getElementById('login-button').click()`, nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression to press login button")
	}

	// Wait for ChromeOS WebAuthn dialog.
	if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(dialog)(ctx); err != nil {
		return errors.Wrap(err, "failed to wait for the ChromeOS dialog")
	}

	if err := authCallback(ctx, ui); err != nil {
		return errors.Wrap(err, "failed to call authCallback")
	}

	if err := CheckGetAssertionSuccessInWebAuthnIo(ctx, conn); err != nil {
		return errors.Wrap(err, "failed to perform GetAssertion")
	}
	return nil
}

// randomUsername returns a random username of length 20.
func randomUsername() string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"

	ret := make([]byte, 20)
	for i := range ret {
		ret[i] = letters[rand.Intn(len(letters))]
	}

	return string(ret)
}

// CheckMakeCredentialSuccessInWebAuthnIo checks Make Credential succeeded by polling js attributes on webauthn.io.
func CheckMakeCredentialSuccessInWebAuthnIo(ctx context.Context, conn *chrome.Conn) error {
	return testing.Poll(ctx, func(context.Context) error {
		result := true
		err := conn.Eval(ctx, `document.getElementsByClassName('alert-success').length > 0`, &result)
		if err != nil {
			return err
		}
		if !result {
			return errors.New("failed to wait for js attribute that should appear after Make Credential")
		}
		return nil

	}, &testing.PollOptions{Timeout: 5 * time.Second})
}

// CheckGetAssertionSuccessInWebAuthnIo checks Get Assertion succeeded by polling whether webauthn.io has
// redirected to the /dashboard path.
func CheckGetAssertionSuccessInWebAuthnIo(ctx context.Context, conn *chrome.Conn) error {
	return testing.Poll(ctx, func(context.Context) error {
		result := true
		// The only component with meaningful className or id appears that can be queried is the party cat.
		err := conn.Eval(ctx, `document.getElementsByClassName('party-cat').length > 0`, &result)
		if err != nil {
			return err
		}
		if !result {
			return errors.New("failed to wait for website to redirect to dashboard after Get Assertion")
		}
		return nil

	}, &testing.PollOptions{Timeout: 10 * time.Second})
}
