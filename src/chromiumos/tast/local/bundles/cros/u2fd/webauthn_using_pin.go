// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package u2fd

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/u2fd/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
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
		Timeout:      5 * time.Minute,
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

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	authCallback := func(ctx context.Context, ui *uiauto.Context) error {
		// Check if the UI is correct.
		var node *nodewith.Finder
		// The accessibility namings of these two corresponding fields are different: one uses specific class name,
		// another uses normal "Views" classname with explicitly set name.
		if autosubmit {
			node = nodewith.Name("Enter your PIN")
		} else {
			node = nodewith.ClassName("LoginPinInputView")
		}
		if err := ui.Exists(node)(ctx); err != nil {
			return errors.Wrap(err, "failed to find the pin input field")
		}
		// Type PIN into ChromeOS WebAuthn dialog. Autosubmitted.
		if err := keyboard.Type(ctx, PIN); err != nil {
			return errors.Wrap(err, "failed to type PIN into ChromeOS auth dialog")
		}
		return nil
	}

	// TODO(b/210418148): Use an internal site for testing to prevent flakiness.
	if err := util.WebAuthnInWebAuthnIo(ctx, cr, authCallback); err != nil {
		s.Fatal("Failed to perform WebAuthn: ", err)
	}
}
