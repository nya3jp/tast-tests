// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/dlp/clipboard"
	"chromiumos/tast/local/bundles/cros/dlp/dragdrop"
	"chromiumos/tast/local/bundles/cros/dlp/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/display"
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
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with drag and drop restrictions from Ash to Lacros and vice versa",
		Contacts: []string{
			"alvinlee@google.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "lacrosPolicyLoggedIn",
		Timeout:      3 * time.Minute,
	})
}

func DataLeakPreventionRulesListDragdropMixedTypeBrowsers(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

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

	// Sets the display zoom factor to minimum, to ensure that the work area
	// length is at least twice the minimum length of a browser window, so that
	// browser windows can be snapped in split view.
	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get the primary display info: ", err)
	}
	zoomInitial := info.DisplayZoomFactor
	zoomMin := info.AvailableDisplayZoomFactors[0]
	if err := display.SetDisplayProperties(ctx, tconn, info.ID, display.DisplayProperties{DisplayZoomFactor: &zoomMin}); err != nil {
		s.Fatalf("Failed to set display zoom factor to minimum %f: %v", zoomMin, err)
	}
	defer display.SetDisplayProperties(cleanupCtx, tconn, info.ID, display.DisplayProperties{DisplayZoomFactor: &zoomInitial})

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
		{
			name:           "blockedLacrosToAsh",
			dropAllowed:    false,
			srcURL:         "www.example.com",
			srcContent:     "Example Domain",
			srcBrowserType: browser.TypeLacros,
			dstBrowserType: browser.TypeAsh,
		},
		{
			name:           "allowedAshToLacros",
			dropAllowed:    true,
			srcURL:         "www.chromium.org",
			srcContent:     "The Chromium Projects",
			srcBrowserType: browser.TypeAsh,
			dstBrowserType: browser.TypeLacros,
		},
		{
			name:           "allowedLacrosToAsh",
			dropAllowed:    true,
			srcURL:         "www.chromium.org",
			srcContent:     "The Chromium Projects",
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

			// Setup destination browser.
			closeDstBr, dstConn, err := openWebsite(ctx, s.FixtValue(), param.dstBrowserType, dstURL)
			if err != nil {
				s.Fatalf("Failed to open %q: %v", dstURL, err)
			}
			defer closeDstBr(cleanupCtx)
			defer dstConn.Close()

			// Setup source browser.
			closeSrcBr, srcConn, err := openWebsite(ctx, s.FixtValue(), param.srcBrowserType, param.srcURL)
			if err != nil {
				s.Fatalf("Failed to open %q: %v", param.srcURL, err)
			}
			defer closeSrcBr(cleanupCtx)
			defer srcConn.Close()

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				s.Fatal("Failed to enter into the overview mode: ", err)
			}

			// Snap the param.srcURL window to the right.
			w1, err := snapFirstWindowInOverview(ctx, tconn, ash.WindowStateRightSnapped)
			if err != nil {
				s.Fatalf("Failed to snap the %s window to the right: %s", param.srcURL, err)
			}

			// Snap the google.com window to the left.
			_, err = snapFirstWindowInOverview(ctx, tconn, ash.WindowStateLeftSnapped)
			if err != nil {
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

			// Check dropped content.
			dropError := dragdrop.CheckDraggedContent(ctx, ui, param.srcContent)

			if param.dropAllowed && dropError != nil {
				s.Error("Checked pasted content but found an error: ", dropError)
			}

			if !param.dropAllowed && dropError == nil {
				s.Error("Content was pasted but should have been blocked")
			}
		})
	}
}

// openWebsite opens a browser of |brType| and navigates to the |url|.
func openWebsite(ctx context.Context, fixture interface{}, brType browser.Type, url string) (func(ctx context.Context), *chrome.Conn, error) {
	br, closeBr, err := browserfixt.SetUp(ctx, fixture, brType)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "couldn't launch the %v browser", brType)
	}

	conn, err := br.NewConn(ctx, "https://"+url)
	if err != nil {
		return closeBr, nil, err
	}

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		return closeBr, conn, errors.Wrapf(err, "%q couldn't achieve quiescence", url)
	}

	return closeBr, conn, nil
}

// snapFirstWindowInOverview sets the first window in the overview to a |targetState|.
func snapFirstWindowInOverview(ctx context.Context, tconn *chrome.TestConn, targetState ash.WindowStateType) (*ash.Window, error) {
	w, err := ash.FindFirstWindowInOverview(ctx, tconn)
	if err != nil {
		return w, err
	}

	if err := ash.SetWindowStateAndWait(ctx, tconn, w.ID, targetState); err != nil {
		return w, err
	}

	return w, nil
}
