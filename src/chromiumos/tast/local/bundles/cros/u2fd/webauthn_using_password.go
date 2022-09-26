// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package u2fd

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/u2fd/util"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         WebauthnUsingPassword,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks that WebAuthn using password succeeds",
		Contacts: []string{
			"hcyang@google.com",
			"cros-hwsec@chromium.org",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "tpm1",
			ExtraSoftwareDeps: []string{"tpm1"},
			// TODO(b/248652978): Fix the regression caused by WebAuthn site changes.
			ExtraAttr: []string{"informational"},
			Fixture:   "chromeLoggedIn",
			Val:       browser.TypeAsh,
		}, {
			Name:              "tpm1_lacros",
			ExtraSoftwareDeps: []string{"tpm1", "lacros"},
			// TODO(b/248652978): Fix the regression caused by WebAuthn site changes.
			ExtraAttr: []string{"informational"},
			Fixture:   "lacros",
			Val:       browser.TypeLacros,
		}, {
			Name:              "gsc",
			ExtraSoftwareDeps: []string{"gsc"},
			// TODO(b/248652978): Fix the regression caused by WebAuthn site changes.
			ExtraAttr: []string{"informational"},
			Fixture:   "chromeLoggedIn",
			Val:       browser.TypeAsh,
		}, {
			Name:              "gsc_lacros",
			ExtraSoftwareDeps: []string{"gsc", "lacros"},
			ExtraAttr:         []string{"informational"},
			Fixture:           "lacros",
			Val:               browser.TypeLacros,
		}},
		Timeout: 5 * time.Minute,
	})
}

func WebauthnUsingPassword(ctx context.Context, s *testing.State) {
	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	bt := s.Param().(browser.Type)

	if err := upstart.CheckJob(ctx, "u2fd"); err != nil {
		s.Fatal("u2fd isn't started: ", err)
	}

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, bt)
	if err != nil {
		s.Fatalf("Failed to open the %v browser: %v", bt, err)
	}
	defer closeBrowser(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to get test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	authCallback := func(ctx context.Context, ui *uiauto.Context) error {
		// Check if the UI is correct.
		if err := ui.Exists(nodewith.ClassName("LoginPasswordView"))(ctx); err != nil {
			return errors.Wrap(err, "failed to find the password input field")
		}
		// Type password into ChromeOS WebAuthn dialog.
		if err := keyboard.Type(ctx, chrome.DefaultPass+"\n"); err != nil {
			return errors.Wrap(err, "failed to type password into ChromeOS auth dialog")
		}
		return nil
	}

	// TODO(b/210418148): Use an internal site for testing to prevent flakiness.
	if err := util.WebAuthnInWebAuthnIo(ctx, cr, br, authCallback); err != nil {
		s.Fatal("Failed to perform WebAuthn: ", err)
	}
}
