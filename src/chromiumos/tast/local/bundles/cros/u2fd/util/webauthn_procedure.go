// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/syslog"
)

// WebAuthnInSecurityKeysInfo performs the WebAuthn procedure in the external site securitykeys.info/qa.html.
func WebAuthnInSecurityKeysInfo(ctx context.Context, cr *chrome.Chrome, authCallback func(context.Context, *uiauto.Context) error) error {
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get test API connection")
	}

	logReader, err := syslog.NewChromeReader(ctx, syslog.ChromeLogFile)
	if err != nil {
		return errors.Wrap(err, "could not get Chrome log reader")
	}
	defer logReader.Close()

	// TODO(b/210418148): Use an internal site for testing to prevent flakiness.
	conn, err := cr.NewConn(ctx, "https://securitykeys.info/qa.html")
	if err != nil {
		return errors.Wrap(err, "failed to navigate to test website")
	}
	defer conn.Close()

	// Perform MakeCredential on the test website.

	// Choose webauthn
	err = conn.Eval(ctx, `document.getElementById('regWebauthn').click()`, nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression")
	}

	// Choose none attestation
	err = conn.Eval(ctx, `document.getElementById('attNone').click()`, nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression")
	}

	// Press "Register" button
	err = conn.Eval(ctx, `document.getElementById('submit').click()`, nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression")
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

	// Press "Authenticate" button. There should be only 1 button in registration-list.
	err = conn.Eval(ctx, `document.getElementById('registration-list').querySelector("button").click()`, nil)
	if err != nil {
		return errors.Wrap(err, "failed to execute JS expression")
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
