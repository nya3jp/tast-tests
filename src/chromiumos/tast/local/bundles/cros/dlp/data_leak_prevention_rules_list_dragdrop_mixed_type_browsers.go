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
	"chromiumos/tast/local/bundles/cros/dlp/dragdrop"
	"chromiumos/tast/local/bundles/cros/dlp/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListDragdropMixedTypeBrowsers,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with drag and drop restrictions from Ash to Lacros and vice versa",
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

func DataLeakPreventionRulesListDragdropMixedTypeBrowsers(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// dstURL := "source.chromium.org"
	dstURL := "google.com"

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
		dropAllowed    bool
		srcURL         string
		srcContent     string
		srcBrowserType browser.Type
		dstBrowserType browser.Type
	}{
		{
			name:           "blockedAshToLacros",
			dropAllowed:    false,
			srcURL:         "www.example.com",
			srcContent:     "Example Domain",
			srcBrowserType: browser.TypeAsh,
			dstBrowserType: browser.TypeLacros,
		},
		// {
		// 	name:           "blockedLacrosToAsh",
		// 	dropAllowed:    false,
		// 	srcURL:         "www.example.com",
		// 	srcContent:     "Example Domain",
		// 	srcBrowserType: browser.TypeLacros,
		// 	dstBrowserType: browser.TypeAsh,
		// },
		// {
		// 	name:           "allowedAshToLacros",
		// 	dropAllowed:    true,
		// 	srcURL:         "www.chromium.org",
		// 	srcContent:     "The Chromium Projects",
		// 	srcBrowserType: browser.TypeAsh,
		// 	dstBrowserType: browser.TypeLacros,
		// },
		// {
		// 	name:           "allowedLacrosToAsh",
		// 	dropAllowed:    true,
		// 	srcURL:         "www.chromium.org",
		// 	srcContent:     "The Chromium Projects",
		// 	srcBrowserType: browser.TypeLacros,
		// 	dstBrowserType: browser.TypeAsh,
		// },
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

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				s.Fatal("Failed to enter into the overview mode: ", err)
			}

			// Snap the param.srcURL window to the right
			w1, err := ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				s.Fatalf("Failed to find the %s window in the overview mode: %s", param.srcURL, err)
			}

			if err := ash.SetWindowStateAndWait(ctx, tconn, w1.ID, ash.WindowStateRightSnapped); err != nil {
				s.Fatalf("Failed to snap the %s window to the right: %s", param.srcURL, err)
			}

			// Snap the google.com window to the left.
			w2, err := ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to find the google.com window in the overview mode: ", err)
			}

			if err := ash.SetWindowStateAndWait(ctx, tconn, w2.ID, ash.WindowStateLeftSnapped); err != nil {
				s.Fatal("Failed to snap the google.com window to the left: ", err)
			}

			// Activate the drag source (param.srcURL) window.
			if err := w1.ActivateWindow(ctx, tconn); err != nil {
				s.Fatalf("Failed to activate the %s window: %s", param.srcURL, err)
			}

			if err = keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			s.Log("Draging and dropping content")
			if err := dragdrop.DragDrop(ctx, tconn, param.srcContent); err != nil {
				s.Fatal("Failed to drag and drop content: ", err)
			}

			s.Log("Checking notification")
			ui := uiauto.New(tconn)
			err = clipboard.CheckClipboardBubble(ctx, ui, param.srcURL)

			if !param.dropAllowed && err != nil {
				s.Error("Couldn't check for notification: ", err)
			}

			if param.dropAllowed && err == nil {
				s.Error("Content pasted, expected restriction")
			}

			// // Check pasted content.
			// pastedError := clipboard.CheckPastedContent(ctx, ui, copiedString)

			// if param.dropAllowed && pastedError != nil {
			// 	s.Error("Checked pasted content but found an error: ", pastedError)
			// }

			// if !param.dropAllowed && pastedError == nil {
			// 	s.Error("Content was pasted but should have been blocked")
			// }
		})
	}
}
