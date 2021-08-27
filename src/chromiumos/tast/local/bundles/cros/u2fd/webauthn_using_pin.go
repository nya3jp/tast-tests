// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package u2fd

import (
	"context"
	"strings"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/cryptohome"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/syslog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: WebauthnUsingPIN,
		Desc: "Checks that WebAuthn using PIN succeeds",
		Contacts: []string{
			"yichengli@chromium.org", // Test author
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

	tconn, err := setUpUserPIN(ctx, cr, PIN, password, autosubmit)
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

	// Open test website in a new tab.
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

	// Choose platform authenticator
	buttonParams := ui.FindParams{Role: ui.RoleTypeButton, Name: "This device"}
	platformAuthenticatorButton, err := ui.FindWithTimeout(ctx, tconn, buttonParams, 2*time.Second)
	if err != nil {
		s.Fatal("Failed to select platform authenticator from transport selection sheet: ", err)
	}
	err = platformAuthenticatorButton.LeftClick(ctx)
	if err != nil {
		s.Fatal("Failed to click button for platform authenticator: ", err)
	}

	// Wait for ChromeOS WebAuthn dialog.
	dialogParams := ui.FindParams{ClassName: "AuthDialogWidget"}
	if err := ui.WaitUntilExists(ctx, tconn, dialogParams, 5*time.Second); err != nil {
		s.Fatal("ChromeOS dialog did not show up: ", err)
	}

	// Type PIN into ChromeOS WebAuthn dialog. Autosubmitted.
	if err := keyboard.Type(ctx, PIN); err != nil {
		s.Fatal("Failed to type PIN into ChromeOS auth dialog: ", err)
	}

	if err := assertMakeCredentialSuccess(ctx, logReader); err != nil {
		s.Fatal("MakeCredential did not succeed: ", err)
	}

	// Perform GetAssertion on the test website.

	// Press "Authenticate" button. There should be only 1 button in registration-list.
	err = conn.Eval(ctx, `document.getElementById('registration-list').querySelector("button").click()`, nil)
	if err != nil {
		s.Fatal("Failed to execute JS expression: ", err)
	}

	// Wait for ChromeOS WebAuthn dialog.
	if err := ui.WaitUntilExists(ctx, tconn, dialogParams, 5*time.Second); err != nil {
		s.Fatal("ChromeOS dialog did not show up: ", err)
	}

	// Type PIN into ChromeOS WebAuthn dialog. Autosubmitted.
	if err := keyboard.Type(ctx, PIN); err != nil {
		s.Fatal("Failed to type PIN into ChromeOS auth dialog: ", err)
	}

	if err := assertGetAssertionSuccess(ctx, logReader); err != nil {
		s.Fatal("GetAssertion did not succeed: ", err)
	}
}

// setUpUserPIN sets up a test user with a specific PIN.
func setUpUserPIN(ctx context.Context, cr *chrome.Chrome, PIN, password string, autosubmit bool) (*chrome.TestConn, error) {
	user := cr.NormalizedUser()
	if mounted, err := cryptohome.IsMounted(ctx, user); err != nil {
		return nil, errors.Wrapf(err, "failed to check mounted vault for %q", user)
	} else if !mounted {
		return nil, errors.Wrapf(err, "no mounted vault for %q", user)
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "getting test API connection failed")
	}

	// Set up PIN through a connection to the Settings page.
	settings, err := ossettings.Launch(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch Settings app")
	}

	if err := settings.EnablePINUnlock(cr, password, PIN, autosubmit)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to enable PIN unlock")
	}

	if err := verifyPINUnlock(ctx, tconn, PIN, autosubmit); err != nil {
		return nil, errors.Wrap(err, "PIN unlock doesn't work so IsUvpaa will be false")
	}

	return tconn, nil
}

func verifyPINUnlock(ctx context.Context, tconn *chrome.TestConn, PIN string, autosubmit bool) error {
	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to lock the screen")
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return st.Locked && st.ReadyForPassword }, 30*time.Second); err != nil {
		return errors.Wrapf(err, "waiting for screen to be locked failed (last status %+v)", st)
	}

	// Enter and submit the PIN to unlock the DUT.
	if err := lockscreen.EnterPIN(ctx, tconn, PIN); err != nil {
		return errors.Wrap(err, "failed to enter in PIN")
	}

	if !autosubmit {
		if err := lockscreen.SubmitPIN(ctx, tconn); err != nil {
			return errors.Wrap(err, "failed to submit PIN")
		}
	}

	if st, err := lockscreen.WaitState(ctx, tconn, func(st lockscreen.State) bool { return !st.Locked }, 30*time.Second); err != nil {
		return errors.Wrapf(err, "waiting for screen to be unlocked failed (last status %+v)", st)
	}
	return nil
}

// assertMakeCredentialSuccess asserts MakeCredential succeeded by looking at Chrome log.
func assertMakeCredentialSuccess(ctx context.Context, logReader *syslog.ChromeReader) error {
	const makeCredentialSuccessLine = "Make credential status: 1"

	if pollErr := testing.Poll(ctx, func(ctx context.Context) error {
		entry, err := logReader.Read()
		if err != nil {
			return err
		}
		if strings.Contains(entry.Content, makeCredentialSuccessLine) {
			return nil
		}
		return errors.New("result not found yet")
	}, &testing.PollOptions{Timeout: 30 * time.Second}); pollErr != nil {
		return errors.Wrap(pollErr, "MakeCredential did not succeed")
	}
	return nil
}

// assertGetAssertionSuccess asserts GetAssertion succeeded by looking at Chrome log.
func assertGetAssertionSuccess(ctx context.Context, logReader *syslog.ChromeReader) error {
	const getAssertionSuccessLine = "GetAssertion status: 1"

	if pollErr := testing.Poll(ctx, func(ctx context.Context) error {
		entry, err := logReader.Read()
		if err != nil {
			return err
		}
		if strings.Contains(entry.Content, getAssertionSuccessLine) {
			return nil
		}
		return errors.New("result not found yet")
	}, &testing.PollOptions{Timeout: 30 * time.Second}); pollErr != nil {
		return pollErr
	}
	return nil
}
