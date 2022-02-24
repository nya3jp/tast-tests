// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/testing"
)

// WebAuthnInWebAuthnIo performs the WebAuthn procedure in the external site webauthn.io.
func WebAuthnInWebAuthnIo(ctx context.Context, cr *chrome.Chrome, authCallback func(context.Context, *uiauto.Context) error) error {
	// We need truly random values for username strings so that different test runs don't affect each other.
	rand.Seed(time.Now().UnixNano())

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test API connection")
	}

	logReader, err := syslog.NewChromeReader(ctx, syslog.ChromeLogFile)
	if err != nil {
		return errors.Wrap(err, "could not get Chrome log reader")
	}
	defer logReader.Close()

	conn, err := cr.NewConn(ctx, "https://webauthn.io/")
	if err != nil {
		return errors.Wrap(err, "failed to navigate to test website")
	}
	defer conn.Close()

	// Perform MakeCredential on the test website.

	// Enter username
	name := randomUsername()
	testing.ContextLogf(ctx, "Username: %s", name)
	// Use a random username because webauthn.io keeps state for each username for a period of time.
	err = conn.Eval(ctx, fmt.Sprintf(`document.getElementById('input-email').value = "%s"`, name), nil)
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
	if err = ui.LeftClick(platformAuthenticatorButton)(ctx); err != nil {
		return errors.Wrap(err, "failed to click button for platform authenticator")
	}

	// Wait for ChromeOS WebAuthn dialog.
	dialog := nodewith.ClassName("AuthDialogWidget")
	if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(dialog)(ctx); err != nil {
		return errors.Wrap(err, "ChromeOS dialog did not show up")
	}

	if err := authCallback(ctx, ui); err != nil {
		return errors.Wrap(err, "failed to call authCallback")
	}

	if err := AssertMakeCredentialSuccess(ctx, logReader); err != nil {
		return errors.Wrap(err, "MakeCredential did not succeed")
	}

	// Perform GetAssertion on the test website.

	// Press "Login" button.
	err = conn.Eval(ctx, `document.getElementById('login-button').click()`, nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression to press login button")
	}

	// Wait for ChromeOS WebAuthn dialog.
	if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(dialog)(ctx); err != nil {
		return errors.Wrap(err, "ChromeOS dialog did not show up")
	}

	if err := authCallback(ctx, ui); err != nil {
		return errors.Wrap(err, "failed to call authCallback")
	}

	if err := AssertGetAssertionSuccess(ctx, logReader); err != nil {
		return errors.Wrap(err, "GetAssertion did not succeed")
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
