// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package u2fd

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/u2fd/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebauthnPINLockout,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that WebAuthn PIN lockouts after too many attempts and fallbacks to password",
		Contacts: []string{
			"hcyang@google.com",
			"cros-hwsec@chromium.org",
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

func WebauthnPINLockout(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	bt := s.Param().(browser.Type)

	if err := upstart.CheckJob(ctx, "u2fd"); err != nil {
		s.Fatal("u2fd isn't started: ", err)
	}

	const (
		username           = fixtures.Username
		password           = fixtures.Password
		PIN                = "123456"
		IncorrectPIN       = "000000"
		autosubmit         = false
		pinLockoutAttempts = 5
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

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, bt)
	if err != nil {
		s.Fatalf("Failed to open the %v browser: %v", bt, err)
	}
	defer closeBrowser(cleanupCtx)

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	tconn, err := util.SetUpUserPIN(ctx, cr, keyboard, PIN, password, autosubmit)
	if err != nil {
		s.Fatal("Failed to set up PIN: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	authCallback := func(ctx context.Context, ui *uiauto.Context) error {
		// Check if the UI is correct.
		var pinInputNode *nodewith.Finder
		// The accessibility namings of these two corresponding fields are different: one uses specific class name,
		// another uses normal "Views" classname with explicitly set name.
		if autosubmit {
			pinInputNode = nodewith.Name("Enter your PIN")
		} else {
			pinInputNode = nodewith.ClassName("LoginPasswordView")
		}
		if err := ui.Exists(pinInputNode)(ctx); err != nil {
			return errors.Wrap(err, "failed to find the pin input field")
		}
		// Type incorrect PIN into ChromeOS WebAuthn dialog. Optionally autosubmitted.
		pinString := IncorrectPIN
		if !autosubmit {
			pinString += "\n"
		}
		for i := 1; i <= pinLockoutAttempts; i++ {
			if err := keyboard.Type(ctx, pinString); err != nil {
				return errors.Wrap(err, "failed to type PIN into ChromeOS auth dialog")
			}
			var errorMsg string
			if i != pinLockoutAttempts {
				errorMsg = "Incorrect PIN"
			} else {
				errorMsg = "Too many attempts"
			}
			node := nodewith.Name(errorMsg)
			if err := ui.WithTimeout(3 * time.Second).WaitUntilExists(node)(ctx); err != nil {
				return errors.Wrapf(err, "failed to wait for %v message", errorMsg)
			}
		}

		// Cancel the WebAuthn dialog.
		node := nodewith.Role(role.Button).Name("Cancel")
		if err = ui.DoDefault(node)(ctx); err != nil {
			return errors.Wrap(err, "failed to cancel the WebAuthn dialog")
		}

		// An error dialog will appear, press "Try again".
		node = nodewith.Role(role.Button).Name("Try again")
		if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(node)(ctx); err != nil {
			return errors.Wrap(err, "failed to wait for the retry button")
		}
		// The retry button shouldn't be continuously clicked with an interval too short, as it restarts the
		// processing on each click.
		if err = ui.WithInterval(time.Second*2).DoDefaultUntil(node, ui.Gone(node))(ctx); err != nil {
			return errors.Wrap(err, "failed to press the retry button")
		}

		// Choose platform authenticator again.
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

		// Password input field should exist, PIN shouldn't because it's locked out.
		passwordInputNode := nodewith.ClassName("LoginPasswordView")
		if err := ui.Exists(passwordInputNode)(ctx); err != nil {
			return errors.Wrap(err, "failed to find the password input field")
		}
		pinPadNode := nodewith.ClassName("LoginPinView")
		if err := ui.Exists(pinPadNode)(ctx); err == nil {
			return errors.Wrap(err, "failed to check the pin pad doesn't exist")
		}

		// Type password into ChromeOS WebAuthn dialog.
		if err := keyboard.Type(ctx, password+"\n"); err != nil {
			return errors.Wrap(err, "failed to type password into ChromeOS auth dialog")
		}
		return nil
	}

	if err := util.WebAuthnInWebAuthnIo(ctx, cr, br, authCallback); err != nil {
		s.Fatal("Failed to perform WebAuthn: ", err)
	}
}
