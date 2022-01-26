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
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListClipboardMixedTypeBrowsers,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "TODO",
		Contacts: []string{
			"alvinlee@google.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "lacrosPolicyLoggedIn",
	})
}

func DataLeakPreventionRulesListClipboardMixedTypeBrowsers(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
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

	s.Log("Waiting for chrome.clipboard API to become available")
	if err := tconn.WaitForExpr(ctx, "chrome.clipboard"); err != nil {
		s.Fatal("chrome.clipboard API unavailable: ", err)
	}

	// Set DLP policy with clipboard blocked restriction.
	if err := policyutil.ServeAndVerify(ctx, fdms, cr, policy.StandardDLPPolicyForClipboard()); err != nil {
		s.Fatal("Failed to serve and verify: ", err)
	}

	for _, param := range []struct {
		name           string
		copyAllowed    bool
		url            string
		srcBrowserType browser.Type
		dstBrowserType browser.Type
	}{
		{
			name:           "example",
			copyAllowed:    false,
			url:            "www.example.com",
			srcBrowserType: browser.TypeAsh,
			dstBrowserType: browser.TypeLacros,
		},
		{
			name:           "chromium",
			copyAllowed:    true,
			url:            "www.chromium.org",
			srcBrowserType: browser.TypeAsh,
			dstBrowserType: browser.TypeLacros,
		},
		{
			name:           "example",
			copyAllowed:    false,
			url:            "www.example.com",
			srcBrowserType: browser.TypeLacros,
			dstBrowserType: browser.TypeAsh,
		},
		{
			name:           "chromium",
			copyAllowed:    true,
			url:            "www.chromium.org",
			srcBrowserType: browser.TypeLacros,
			dstBrowserType: browser.TypeAsh,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Setup source browser.
			srcBr, closeSrcBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), param.srcBrowserType)
			if err != nil {
				s.Fatal("Failed to open the source browser: ", err)
			}
			defer closeSrcBrowser(cleanupCtx)

			// Setup destination browser.
			dstBr, closeDstBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), param.dstBrowserType)
			if err != nil {
				s.Fatal("Failed to open the destination browser: ", err)
			}
			defer closeDstBrowser(cleanupCtx)

			conn, err := srcBr.NewConn(ctx, "https://"+param.url)
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

			googleConn, err := dstBr.NewConn(ctx, "https://www.google.com/?hl=en")
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
