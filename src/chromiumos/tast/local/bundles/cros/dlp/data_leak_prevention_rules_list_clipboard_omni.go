// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/dlp/clipboard"
	"chromiumos/tast/local/bundles/cros/dlp/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListClipboardOmni,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with clipboard blocked restriction with omni box",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
	})
}

func DataLeakPreventionRulesListClipboardOmni(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Set DLP policy with all clipboard blocked restriction.
	if err := policyutil.ServeAndVerify(ctx, fakeDMS, cr, policy.RestrictiveDLPPolicyForClipboard()); err != nil {
		s.Fatal("Failed to serve and verify: ", err)
	}

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	s.Log("Waiting for chrome.clipboard API to become available")
	if err := tconn.WaitForExpr(ctx, "chrome.clipboard"); err != nil {
		s.Fatal("chrome.clipboard API unavailable: ", err)
	}

	for _, param := range []struct {
		name        string
		url         string
		wantAllowed bool
	}{
		{
			name:        "example",
			url:         "www.example.com",
			wantAllowed: false,
		},
		{
			name:        "chromium",
			url:         "www.chromium.org",
			wantAllowed: true,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := cr.ResetState(ctx); err != nil {
				s.Fatal("Failed to reset the Chrome: ", err)
			}

			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(cleanupCtx)

			conn, err := br.NewConn(ctx, "https://"+param.url)
			if err != nil {
				s.Fatal("Failed to open page: ", err)
			}
			defer conn.Close()

			if err := keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+C"); err != nil {
				s.Fatal("Failed to press Ctrl+C to copy content: ", err)
			}

			err = rightClickOmnibox(ctx, tconn, param.url, param.wantAllowed)
			if err != nil {
				s.Fatal("Failed to right click omni box: ", err)
			}

			// Lacros variant doesn't work correctly without dismissing the right click menu first (it doesn't react to "Ctrl+T").
			if err := uiauto.Combine("open a new tab",
				keyboard.AccelAction("Esc"), // Dismiss the right click menu.
				keyboard.AccelAction("Ctrl+T"))(ctx); err != nil {
				s.Fatal("Failed to press Ctrl+T to open a new tab: ", err)
			}

			err = pasteOmnibox(ctx, tconn, keyboard, param.url, param.wantAllowed)
			if err != nil {
				s.Fatal("Failed to paste content in omni box: ", err)
			}
		})
	}
}

func rightClickOmnibox(ctx context.Context, tconn *chrome.TestConn, url string, wantAllowed bool) error {
	ui := uiauto.New(tconn)

	addressBar := nodewith.HasClass("OmniboxViewViews")

	if err := ui.RightClick(addressBar)(ctx); err != nil {
		return errors.Wrap(err, "failed to right click omni box")
	}

	err := clipboard.CheckGreyPasteNode(ctx, ui)
	if err != nil && !wantAllowed {
		return err
	}

	if err == nil && wantAllowed {
		return errors.New("Paste node found greyed, expected focusable")
	}

	// Clipboard DLP bubble is never expected on right click.
	err = clipboard.CheckClipboardBubble(ctx, ui, url)
	if err == nil {
		return errors.New("Notification found, expected none")
	}

	return nil
}

func pasteOmnibox(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, url string, wantAllowed bool) error {
	ui := uiauto.New(tconn)

	if err := uiauto.Combine("Paste content in Omnibox",
		keyboard.AccelAction("Ctrl+L"),
		keyboard.AccelAction("Ctrl+V"))(ctx); err != nil {
		return errors.Wrap(err, "failed to paste content in Omnibox")
	}

	err := clipboard.CheckClipboardBubble(ctx, ui, url)
	if err != nil && !wantAllowed {
		return err
	}

	if err == nil && wantAllowed {
		return errors.New("Notification found, expected none")
	}

	return nil
}
