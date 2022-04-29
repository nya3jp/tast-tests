// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/bundles/cros/dlp/clipboard"
	"chromiumos/tast/local/bundles/cros/dlp/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/chrome/webutil"
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
			"ayaelattar@google.com",
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
		Data:         []string{"text_1.html", "text_2.html", "editable_text_box.html"},
	})
}

func DataLeakPreventionRulesListClipboard(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	allowedServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer allowedServer.Close()

	blockedServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer blockedServer.Close()

	destServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer destServer.Close()

	if err := policyutil.ServeAndVerify(ctx, fakeDMS, cr, policy.PopulateDLPPolicyForClipboard(blockedServer.URL, destServer.URL)); err != nil {
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
		sourceURL   string
	}{
		{
			name:        "copyBlocked",
			copyAllowed: false,
			sourceURL:   blockedServer.URL + "/text_1.html",
		},
		{
			name:        "copyAllowed",
			copyAllowed: true,
			sourceURL:   allowedServer.URL + "/text_2.html",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			sourceConn, err := cr.NewConn(ctx, param.sourceURL)
			if err != nil {
				s.Fatalf("Failed to open page %q: %v", param.sourceURL, err)
			}
			defer sourceConn.Close()

			if err := webutil.WaitForQuiescence(ctx, sourceConn, 10*time.Second); err != nil {
				s.Fatalf("Failed to wait for %q to achieve quiescence: %v", param.sourceURL, err)
			}

			if err := uiauto.Combine("copy all text from source website",
				keyboard.AccelAction("Ctrl+A"),
				keyboard.AccelAction("Ctrl+C"))(ctx); err != nil {
				s.Fatal("Failed to copy text from source browser: ", err)
			}

			copiedString, err := clipboard.GetClipboardContent(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get clipboard content: ", err)
			}

			destURL := destServer.URL + "/editable_text_box.html"
			destConn, err := cr.NewConn(ctx, destURL)
			if err != nil {
				s.Fatalf("Failed to open page %q: %v", destURL, err)
			}
			defer destConn.Close()

			if err := webutil.WaitForQuiescence(ctx, destConn, 10*time.Second); err != nil {
				s.Fatalf("Failed to wait for %q to achieve quiescence: %v", destURL, err)
			}

			ui := uiauto.New(tconn)

			textBoxNode := nodewith.Name("textarea").Role(role.TextField).State(state.Editable, true).First()
			if err := uiauto.Combine("Pasting into text box",
				ui.WaitUntilExists(textBoxNode.Visible()),
				ui.LeftClick(textBoxNode),
				ui.WaitUntilExists(textBoxNode.Focused()),
				keyboard.AccelAction("Ctrl+V"),
			)(ctx); err != nil {
				s.Fatal("Failed to paste into text box: ", err)
			}

			// Verify notification bubble.
			parsedSourceURL, _ := url.Parse(blockedServer.URL)
			notifError := clipboard.CheckClipboardBubble(ctx, ui, parsedSourceURL.Hostname())

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
