// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package u2fd

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/bundles/cros/u2fd/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebauthnUsingPIN,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Checks that WebAuthn using PIN succeeds",
		Contacts: []string{
			"hcyang@google.com",
			"cros-hwsec@chromium.org",
			"martinkr@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "gsc"},
	})
}

func WebauthnUsingPIN(ctx context.Context, s *testing.State) {
	if err := upstart.CheckJob(ctx, "u2fd"); err != nil {
		s.Fatal("u2fd isn't started: ", err)
	}

	// Try to get the system into a consistent state, since it seems like having
	// an already-mounted user dir can cause problems: https://crbug.com/963084
	s.Log("Restarting ui job")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed to restart ui job: ", err)
	}

	const (
		username   = fixtures.Username
		password   = fixtures.Password
		PIN        = "123456"
		autosubmit = true
	)

	fdms, err := fakedms.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start FakeDMS: ", err)
	}
	defer fdms.Stop(ctx)

	if err := fdms.WritePolicyBlob(fakedms.NewPolicyBlob()); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	opts := []chrome.Option{
		chrome.FakeLogin(chrome.Creds{User: username, Pass: password}),
		// Enable device event log in Chrome logs for validation.
		chrome.ExtraArgs("--vmodule=device_event_log*=1"),
		chrome.DMSPolicy(fdms.URL)}
	cr, err := chrome.New(ctx, opts...)
	if err != nil {
		s.Fatal("Failed to log in by Chrome: ", err)
	}
	defer cr.Close(ctx)

	pinPolicies := []policy.Policy{
		&policy.QuickUnlockModeAllowlist{Val: []string{"PIN"}},
		&policy.PinUnlockAutosubmitEnabled{Val: true}}

	if err := policyutil.ServeAndVerify(ctx, fdms, cr, pinPolicies); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	tconn, err := util.SetUpUserPIN(ctx, cr, PIN, password, autosubmit)
	if err != nil {
		s.Fatal("Failed to set up PIN: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	logReader, err := syslog.NewChromeReader(ctx, syslog.ChromeLogFile)
	if err != nil {
		s.Fatal("Could not get Chrome log reader: ", err)
	}
	defer logReader.Close()

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	// TODO(b/210418148): Use an internal site for testing to prevent flakiness.
	conn, err := cr.NewConn(ctx, "https://securitykeys.info/qa.html")
	if err != nil {
		s.Fatal("Failed to navigate to test website: ", err)
	}
	defer conn.Close()

	// Perform MakeCredential on the test website.

	// Choose webauthn
	err = conn.Eval(ctx, `document.getElementById('regWebauthn').click()`, nil)
	if err != nil {
		s.Fatal("Failed to execute JS expression: ", err)
	}

	// Choose none attestation
	err = conn.Eval(ctx, `document.getElementById('attNone').click()`, nil)
	if err != nil {
		s.Fatal("Failed to execute JS expression: ", err)
	}

	// Press "Register" button
	err = conn.Eval(ctx, `document.getElementById('submit').click()`, nil)
	if err != nil {
		s.Fatal("Failed to execute JS expression: ", err)
	}

	ui := uiauto.New(tconn)

	// Choose platform authenticator
	platformAuthenticatorButton := nodewith.Role(role.Button).Name("This device")
	if err := ui.WithTimeout(2 * time.Second).WaitUntilExists(platformAuthenticatorButton)(ctx); err != nil {
		s.Fatal("Failed to select platform authenticator from transport selection sheet: ", err)
	}
	if err = ui.LeftClick(platformAuthenticatorButton)(ctx); err != nil {
		s.Fatal("Failed to click button for platform authenticator: ", err)
	}

	// Wait for ChromeOS WebAuthn dialog.
	dialog := nodewith.ClassName("AuthDialogWidget")
	if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(dialog)(ctx); err != nil {
		s.Fatal("ChromeOS dialog did not show up: ", err)
	}

	// Type PIN into ChromeOS WebAuthn dialog. Autosubmitted.
	if err := keyboard.Type(ctx, PIN); err != nil {
		s.Fatal("Failed to type PIN into ChromeOS auth dialog: ", err)
	}

	if err := util.AssertMakeCredentialSuccess(ctx, logReader); err != nil {
		s.Fatal("MakeCredential did not succeed: ", err)
	}

	// Perform GetAssertion on the test website.

	// Press "Authenticate" button. There should be only 1 button in registration-list.
	err = conn.Eval(ctx, `document.getElementById('registration-list').querySelector("button").click()`, nil)
	if err != nil {
		s.Fatal("Failed to execute JS expression: ", err)
	}

	// Wait for ChromeOS WebAuthn dialog.
	if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(dialog)(ctx); err != nil {
		s.Fatal("ChromeOS dialog did not show up: ", err)
	}

	// Type PIN into ChromeOS WebAuthn dialog. Autosubmitted.
	if err := keyboard.Type(ctx, PIN); err != nil {
		s.Fatal("Failed to type PIN into ChromeOS auth dialog: ", err)
	}

	if err := util.AssertGetAssertionSuccess(ctx, logReader); err != nil {
		s.Fatal("GetAssertion did not succeed: ", err)
	}
}
