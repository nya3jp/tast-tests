// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/dlp/clipboard"
	"chromiumos/tast/local/bundles/cros/dlp/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListClipboardMixedTypeBrowsers,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with clipboard blocked restrictions from Ash to Lacros and vice versa",
		Contacts: []string{
			"alvinlee@google.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "lacrosPolicyLoggedIn",
	})
}

func DataLeakPreventionRulesListClipboardMixedTypeBrowsers(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	dstURL := "source.chromium.org"

	// Reserve 10 seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

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

	for _, param := range []struct {
		name           string
		copyAllowed    bool
		srcURL         string
		srcBrowserType browser.Type
		dstBrowserType browser.Type
	}{
		{
			name:           "blockedAshToLacros",
			copyAllowed:    false,
			srcURL:         "www.example.com",
			srcBrowserType: browser.TypeAsh,
			dstBrowserType: browser.TypeLacros,
		},
		{
			name:           "blockedLacrosToAsh",
			copyAllowed:    false,
			srcURL:         "www.example.com",
			srcBrowserType: browser.TypeLacros,
			dstBrowserType: browser.TypeAsh,
		},
		{
			name:           "allowedAshToLacros",
			copyAllowed:    true,
			srcURL:         "www.chromium.org",
			srcBrowserType: browser.TypeAsh,
			dstBrowserType: browser.TypeLacros,
		},
		{
			name:           "allowedLacrosToAsh",
			copyAllowed:    true,
			srcURL:         "www.chromium.org",
			srcBrowserType: browser.TypeLacros,
			dstBrowserType: browser.TypeAsh,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			if err := policyutil.ServeAndVerify(ctx, fdms, cr, policy.PopulateDLPPolicyForClipboard("example.com", dstURL)); err != nil {
				s.Fatal("Failed to serve and verify the DLP policy: ", err)
			}

			s.Log("Waiting for chrome.clipboard API to become available")
			if err := tconn.WaitForExpr(ctx, "chrome.clipboard"); err != nil {
				s.Fatal("Failed to wait for chrome.clipboard API to become available: ", err)
			}

			// Setup source browser.
			srcBr, closeSrcBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), param.srcBrowserType)
			if err != nil {
				s.Fatalf("Failed to open the %s source browser: %s", param.srcBrowserType, err)
			}
			defer closeSrcBrowser(cleanupCtx)

			conn, err := srcBr.NewConn(ctx, "https://"+param.srcURL)
			if err != nil {
				s.Fatal("Failed to open page: ", err)
			}
			defer conn.Close()

			if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
				s.Fatalf("Failed to wait for %q to achieve quiescence: %v", param.srcURL, err)
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

			// Setup destination browser.
			dstBr, closeDstBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), param.dstBrowserType)
			if err != nil {
				s.Fatalf("Failed to open the %s destination browser: %s", param.dstBrowserType, err)
			}
			defer closeDstBrowser(cleanupCtx)

			dstConn, err := dstBr.NewConn(ctx, "https://"+dstURL)
			if err != nil {
				s.Fatal("Failed to open page: ", err)
			}
			defer dstConn.Close()

			if err := webutil.WaitForQuiescence(ctx, dstConn, 10*time.Second); err != nil {
				s.Fatalf("Failed to wait for %q to achieve quiescence: %v", dstURL, err)
			}

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			ui := uiauto.New(tconn)

			searchNode := nodewith.Name("Search for code or files").Role(role.TextField).State(state.Editable, true).First()

			if err := uiauto.Combine("pasting into search bar",
				ui.WithTimeout(10*time.Second).WaitUntilExists(searchNode),
				ui.WaitUntilExists(searchNode.State(state.Invisible, false)),
				ui.LeftClick(searchNode),
				keyboard.AccelAction("Ctrl+V"),
			)(ctx); err != nil {
				s.Fatal("Failed to paste into search bar: ", err)
			}

			// Verify notification bubble.
			notifError := clipboard.CheckClipboardBubble(ctx, ui, param.srcURL)

			if !param.copyAllowed && notifError != nil {
				s.Error("Expected notification but found an error: ", notifError)
			}

			if param.copyAllowed && notifError == nil {
				s.Error("Didn't expect notification but one was found")
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
