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

	// Set DLP policy with clipboard blocked restriction.
	if err := policyutil.ServeAndVerify(ctx, fakeDMS, cr, policy.StandardDLPPolicyForClipboard()); err != nil {
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
		copyAllowed bool
		url         string
	}{
		{
			name:        "example",
			copyAllowed: false,
			url:         "www.example.com",
		},
		{
			name:        "chromium",
			copyAllowed: true,
			url:         "www.chromium.org",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			conn, err := cr.NewConn(ctx, "https://"+param.url)
			if err != nil {
				s.Error("Failed to open page: ", err)
			}
			defer conn.Close()

			if err := keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+C"); err != nil {
				s.Fatal("Failed to press Ctrl+C to copy content: ", err)
			}

			copiedString, err := clipboard.GetClipboardContent(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get clipboard content: ", err)
			}

			googleConn, err := cr.NewConn(ctx, "https://www.google.com/?hl=en")
			if err != nil {
				s.Error("Failed to open page: ", err)
			}
			defer googleConn.Close()

			ui := uiauto.New(tconn)

			searchNode := nodewith.Name("Search").Role(role.TextFieldWithComboBox).State("editable", true).First()
			// Focus in search box
			if err := ui.LeftClick(searchNode)(ctx); err != nil {
				s.Fatal("Failed to left click node: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+V"); err != nil {
				s.Fatal("Failed to press Ctrl+V to paste content: ", err)
			}

			// Verify Notification Bubble.
			notification := clipboard.CheckClipboardBubble(ctx, ui, param.url)

			if !param.copyAllowed && notification != nil {
				s.Fatal("Couldn't check for notification: ", notification)
			}

			// Check Pasted content.
			pastedError := clipboard.CheckPastedContent(ctx, ui, copiedString)

			if param.copyAllowed && pastedError != nil {
				s.Fatal("Couldn't check for pasted content: ", pastedError)
			}

			if (!param.copyAllowed && pastedError == nil) || (param.copyAllowed && notification == nil) {
				s.Fatal("Content pasted, expected restriction")
			}
		})
	}
}
