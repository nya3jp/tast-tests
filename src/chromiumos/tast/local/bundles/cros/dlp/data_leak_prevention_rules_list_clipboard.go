// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/bundles/cros/dlp/clipboard"
	"chromiumos/tast/local/bundles/cros/dlp/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListClipboard,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with clipboard blocked restriction by copy and paste",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func DataLeakPreventionRulesListClipboard(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	if err := policyutil.ServeAndVerify(ctx, fakeDMS, cr, policy.StandardDLPPolicyForClipboard()); err != nil {
		s.Fatal("Failed to serve and verify the DLP policy: ", err)
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
		s.Fatal("Failed to wait for chrome.clipboard API to become available: ", err)
	}

	for _, param := range []struct {
		name        string
		copyAllowed bool
		url         string
	}{
		{
			name:        "copyBlocked",
			copyAllowed: false,
			url:         "www.example.com",
		},
		{
			name:        "copyAllowed",
			copyAllowed: true,
			url:         "www.chromium.org",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			conn, err := cr.NewConn(ctx, "https://"+param.url)
			if err != nil {
				s.Fatalf("Failed to open page %q: %v", param.url, err)
			}
			defer conn.Close()

			if err := uiauto.Combine("copy all text from source website",
				keyboard.AccelAction("Ctrl+A"),
				keyboard.AccelAction("Ctrl+C"))(ctx); err != nil {
				s.Fatal("Failed to copy text from source browser: ", err)
			}

			copiedString, err := clipboard.GetClipboardContent(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get clipboard content: ", err)
			}

			googleConn, err := cr.NewConn(ctx, "https://www.google.com/?hl=en")
			if err != nil {
				s.Fatal("Failed to open page: ", err)
			}
			defer googleConn.Close()

			ui := uiauto.New(tconn)

			searchNode := nodewith.Name("Search").Role(role.TextFieldWithComboBox).State(state.Editable, true).First()

			if err := ui.WaitUntilExists(searchNode)(ctx); err != nil {
				s.Fatal("Failed to find search bar: ", err)
			}

			searchNodeInfo, err := ui.Info(ctx, searchNode)
			if err != nil {
				s.Fatal("Error retrieving info for search node: ", err)
			}

			// If the search bar is invisible, it is probably overlaid by the Google consent banner.
			// It does not provide usable ids, so we try to quit it with Shift+Tab, then Enter.
			if searchNodeInfo.State[state.Invisible] {
				s.Log("Search bar is invisible, closing consent banner")
				if err := keyboard.Accel(ctx, "Shift+Tab+Enter"); err != nil {
					s.Fatal("Failed to press Shift+Tab+Enter to close consent banner: ", err)
				}
			}

			if err := uiauto.Combine("Pasting into search bar",
				ui.WaitUntilExists(searchNode.Visible()),
				ui.LeftClick(searchNode),
				ui.WaitUntilExists(searchNode.Focused()),
				keyboard.AccelAction("Ctrl+V"),
			)(ctx); err != nil {
				s.Fatal("Failed to paste into search bar: ", err)
			}

			// Verify notification bubble.
			notifError := clipboard.CheckClipboardBubble(ctx, ui, param.url)

			if !param.copyAllowed && notifError != nil {
				s.Error("Expected notification but found an error: ", notifError)
			}

			if param.copyAllowed && notifError == nil {
				s.Error("Didn't expect notification but one was found: ")
			}

			// Check pasted content.
			pastedError := clipboard.CheckPastedContent(ctx, ui, copiedString)

			if param.copyAllowed && pastedError != nil {
				s.Error("Checked pasted content but found an error: ", pastedError)
			}

			if !param.copyAllowed && pastedError == nil {
				s.Error("Content was pasted but should have been blocked")
			}
		})
	}
}
