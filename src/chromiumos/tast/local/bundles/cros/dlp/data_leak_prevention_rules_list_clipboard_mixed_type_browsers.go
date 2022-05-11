// Copyright 2022 The Chromium OS Authors. All rights reserved.
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
		Data:         []string{"text_1.html", "text_2.html", "editable_text_box.html"},
		Fixture:      "lacrosPolicyLoggedIn",
		Timeout:      3 * time.Minute,
	})
}

func DataLeakPreventionRulesListClipboardMixedTypeBrowsers(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	allowedServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer allowedServer.Close()

	blockedServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer blockedServer.Close()

	dstServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer dstServer.Close()

	dstURL := dstServer.URL + "/editable_text_box.html"

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
			srcURL:         blockedServer.URL + "/text_1.html",
			srcBrowserType: browser.TypeAsh,
			dstBrowserType: browser.TypeLacros,
		},
		{
			name:           "blockedLacrosToAsh",
			copyAllowed:    false,
			srcURL:         blockedServer.URL + "/text_1.html",
			srcBrowserType: browser.TypeLacros,
			dstBrowserType: browser.TypeAsh,
		},
		{
			name:           "allowedAshToLacros",
			copyAllowed:    true,
			srcURL:         allowedServer.URL + "/text_2.html",
			srcBrowserType: browser.TypeAsh,
			dstBrowserType: browser.TypeLacros,
		},
		{
			name:           "allowedLacrosToAsh",
			copyAllowed:    true,
			srcURL:         allowedServer.URL + "/text_2.html",
			srcBrowserType: browser.TypeLacros,
			dstBrowserType: browser.TypeAsh,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Reserve time for cleanup.
			cleanupCtx := ctx
			ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
			defer cancel()

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			if err := policyutil.ServeAndVerify(ctx, fdms, cr, policy.PopulateDLPPolicyForClipboard(blockedServer.URL, dstServer.URL)); err != nil {
				s.Fatal("Failed to serve and verify the DLP policy: ", err)
			}

			s.Log("Waiting for chrome.clipboard API to become available")
			if err := tconn.WaitForExpr(ctx, "chrome.clipboard"); err != nil {
				s.Fatal("Failed to wait for chrome.clipboard API to become available: ", err)
			}

			// Setup source browser.
			srcBr, closeSrcBrowser, err := browserfixt.SetUp(ctx, cr, param.srcBrowserType)
			if err != nil {
				s.Fatalf("Failed to open the %s source browser: %s", param.srcBrowserType, err)
			}
			defer closeSrcBrowser(cleanupCtx)

			conn, err := srcBr.NewConn(ctx, param.srcURL)
			if err != nil {
				s.Fatalf("Failed to open page %q: %v", param.srcURL, err)
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
			dstBr, closeDstBrowser, err := browserfixt.SetUp(ctx, cr, param.dstBrowserType)
			if err != nil {
				s.Fatalf("Failed to open the %s destination browser: %s", param.dstBrowserType, err)
			}
			defer closeDstBrowser(cleanupCtx)

			dstConn, err := dstBr.NewConn(ctx, dstURL)
			if err != nil {
				s.Fatalf("Failed to open page %q: %v", dstURL, err)
			}
			defer dstConn.Close()

			if err := webutil.WaitForQuiescence(ctx, dstConn, 10*time.Second); err != nil {
				s.Fatalf("Failed to wait for %q to achieve quiescence: %v", dstURL, err)
			}

			ui := uiauto.New(tconn)

			textBoxNode := nodewith.Name("textarea").Role(role.TextField).State(state.Editable, true).First()
			if err := uiauto.Combine("pasting into text box",
				ui.WaitUntilExists(textBoxNode.Visible()),
				ui.LeftClick(textBoxNode),
				ui.WaitUntilExists(textBoxNode.Focused()),
				keyboard.AccelAction("Ctrl+V"),
			)(ctx); err != nil {
				s.Fatal("Failed to paste into text box: ", err)
			}

			// Verify notification bubble.
			parsedSrcURL, _ := url.Parse(blockedServer.URL)
			notifError := clipboard.CheckClipboardBubble(ctx, ui, parsedSrcURL.Hostname())

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
