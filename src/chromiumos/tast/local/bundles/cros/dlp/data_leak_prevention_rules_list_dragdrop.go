// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
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
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListDragdrop,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with clipboard blocked restriction by drag and drop",
		Contacts: []string{
			"ayaelattar@google.com",
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"text_1.html", "text_2.html", "editable_text_box.html"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}}})
}

func DataLeakPreventionRulesListDragdrop(ctx context.Context, s *testing.State) {
	// Reserve time for various cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	allowedServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer allowedServer.Close()

	blockedServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer blockedServer.Close()

	dstServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer dstServer.Close()

	if err := policyutil.ServeAndVerify(ctx, fakeDMS, cr, policy.PopulateDLPPolicyForClipboard(blockedServer.URL, dstServer.URL)); err != nil {
		s.Fatal("Failed to serve and verify the DLP policy: ", err)
	}

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

	defer ash.SetOverviewModeAndWait(cleanupCtx, tconn, false)
	for _, param := range []struct {
		name        string
		wantAllowed bool
		srcURL      string
		content     string
	}{
		{
			name:        "dropBlocked",
			wantAllowed: false,
			srcURL:      blockedServer.URL + "/text_1.html",
			content:     "Sample text about random things.",
		},
		{
			name:        "dropAllowed",
			wantAllowed: true,
			srcURL:      allowedServer.URL + "/text_2.html",
			content:     "Here is a random piece of text for testing things.",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			if err := cr.ResetState(ctx); err != nil {
				s.Fatal("Failed to reset the Chrome: ", err)
			}

			br, closeBr, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the destination browser: ", err)
			}
			defer closeBr(cleanupCtx)

			dstURL := dstServer.URL + "/editable_text_box.html"
			dstConn, err := br.NewConn(ctx, dstURL)
			if err != nil {
				s.Fatalf("Failed to open page %q: %v", dstURL, err)
			}
			defer dstConn.Close()

			if err := webutil.WaitForQuiescence(ctx, dstConn, 10*time.Second); err != nil {
				s.Fatalf("Failed to wait for %q to achieve quiescence: %v", dstURL, err)
			}

			srcConn, err := br.NewConn(ctx, param.srcURL, browser.WithNewWindow())
			if err != nil {
				s.Fatalf("Failed to open page %q: %v", param.srcURL, err)
			}
			defer srcConn.Close()

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := webutil.WaitForQuiescence(ctx, srcConn, 10*time.Second); err != nil {
				s.Fatalf("Failed to wait for %q to achieve quiescence: %v", param.srcURL, err)
			}

			if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				s.Fatal("Failed to enter into the overview mode: ", err)
			}

			// Snap the param.srcURL window to the right.
			w1, err := ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				s.Fatalf("Failed to find the %s window in the overview mode: %s", param.srcURL, err)
			}

			if err := ash.SetWindowStateAndWait(ctx, tconn, w1.ID, ash.WindowStateRightSnapped); err != nil {
				s.Fatalf("Failed to snap the %s window to the right: %s", param.srcURL, err)
			}

			// Snap the destination window to the left.
			w2, err := ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				s.Fatalf("Failed to find the %s window in the overview mode: %s", dstURL, err)
			}

			if err := ash.SetWindowStateAndWait(ctx, tconn, w2.ID, ash.WindowStateLeftSnapped); err != nil {
				s.Fatalf("Failed to snap the %s window to the left: %s", dstURL, err)
			}

			// Activate the drag source (param.srcURL) window.
			if err := w1.ActivateWindow(ctx, tconn); err != nil {
				s.Fatalf("Failed to activate the %s window: %s", param.srcURL, err)
			}

			if err = keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			s.Log("Draging and dropping content")
			if err := dragdrop.DragDrop(ctx, tconn, param.content); err != nil {
				s.Error("Failed to drag drop content: ", err)
			}

			s.Log("Checking notification")
			ui := uiauto.New(tconn)

			// Verify notification bubble.
			parsedSrcURL, err := url.Parse(blockedServer.URL)
			if err != nil {
				s.Fatalf("Failed to parse blocked server URL %s: %s", blockedServer.URL, err)
			}

			notifError := clipboard.CheckClipboardBubble(ctx, ui, parsedSrcURL.Hostname())

			if !param.wantAllowed && notifError != nil {
				s.Error("Expected notification but found an error: ", notifError)
			}

			if param.wantAllowed && notifError == nil {
				s.Error("Didn't expect notification but one was found: ")
			}

			// Check dropped content.
			dropError := dragdrop.CheckDraggedContent(ctx, ui, param.content)

			if param.wantAllowed && dropError != nil {
				s.Error("Checked pasted content but found an error: ", dropError)
			}

			if !param.wantAllowed && dropError == nil {
				s.Error("Content was pasted but should have been blocked")
			}
		})
	}
}
