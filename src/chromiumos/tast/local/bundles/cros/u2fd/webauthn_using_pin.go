// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package u2fd

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/u2fd/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
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
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that WebAuthn using PIN succeeds",
		Contacts: []string{
			"hcyang@google.com",
			"cros-hwsec@chromium.org",
			"martinkr@chromium.org",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "gsc"},
		Timeout:      5 * time.Minute,
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
	})
}

func WebauthnUsingPIN(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	bt := s.Param().(browser.Type)

	if err := upstart.CheckJob(ctx, "u2fd"); err != nil {
		s.Fatal("u2fd isn't started: ", err)
	}

	const (
		username   = fixtures.Username
		password   = fixtures.Password
		PIN        = "123456"
		autosubmit = false
	)

	pinPolicies := []policy.Policy{
		&policy.QuickUnlockModeAllowlist{Val: []string{"PIN"}},
		&policy.PinUnlockAutosubmitEnabled{Val: autosubmit}}

	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	if err := policyutil.ServeAndVerify(ctx, fdms, cr, pinPolicies); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
	if err != nil {
		s.Fatalf("Failed to open the %v browser: %v", bt, err)
	}
	defer closeBrowser(ctx)

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
			node = nodewith.ClassName("LoginPasswordView")
		}
		if err := ui.Exists(node)(ctx); err != nil {
			return errors.Wrap(err, "failed to find the pin input field")
		}
		// Type PIN into ChromeOS WebAuthn dialog. Optionally autosubmitted.
		pinString := PIN
		if !autosubmit {
			pinString += "\n"
		}
		if err := keyboard.Type(ctx, pinString); err != nil {
			return errors.Wrap(err, "failed to type PIN into ChromeOS auth dialog")
		}
		return nil
	}

	// TODO(b/210418148): Use an internal site for testing to prevent flakiness.
	if err := util.WebAuthnInWebAuthnIo(ctx, cr, br, authCallback); err != nil {
		s.Fatal("Failed to perform WebAuthn: ", err)
	}
}
