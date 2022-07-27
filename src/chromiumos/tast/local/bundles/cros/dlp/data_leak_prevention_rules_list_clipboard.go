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

	"chromiumos/tast/common/fixture"
	policyBlob "chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/dlp/clipboard"
	"chromiumos/tast/local/bundles/cros/dlp/policy"
	"chromiumos/tast/local/bundles/cros/dlp/restrictionlevel"
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
)

// A struct containing parameters for different clipboard tests.
type clipboardTestParams struct {
	name        string
	restriction restrictionlevel.RestrictionLevel
	copyAllowed bool
	browserType browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListClipboard,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with clipboard warning/block restriction by copy and paste",
		Contacts: []string{
			"ayaelattar@google.com",
			"accorsi@google.com",
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"text_1.html", "text_2.html", "editable_text_box.html"},
		Params: []testing.Param{{
			Name:    "ash_blocked",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: clipboardTestParams{
				name:        "blocked",
				restriction: restrictionlevel.Blocked,
				copyAllowed: false,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_allowed",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: clipboardTestParams{
				name:        "allowed",
				restriction: restrictionlevel.Allowed,
				copyAllowed: true,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_warn_proceeded",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: clipboardTestParams{
				name:        "warn_proceded",
				restriction: restrictionlevel.WarnProceeded,
				copyAllowed: true,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_warn_cancelled",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: clipboardTestParams{
				name:        "warn_cancelled",
				restriction: restrictionlevel.WarnCancelled,
				copyAllowed: false,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:              "lacros_blocked",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: clipboardTestParams{
				name:        "blocked",
				restriction: restrictionlevel.Blocked,
				copyAllowed: false,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_allowed",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: clipboardTestParams{
				name:        "allowed",
				restriction: restrictionlevel.Allowed,
				copyAllowed: true,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_proceeded",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: clipboardTestParams{
				name:        "warn_proceeded",
				restriction: restrictionlevel.WarnProceeded,
				copyAllowed: true,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_cancelled",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: clipboardTestParams{
				name:        "warn_cancelled",
				restriction: restrictionlevel.WarnCancelled,
				copyAllowed: false,
				browserType: browser.TypeLacros,
			},
		}},
	})
}

func DataLeakPreventionRulesListClipboard(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	sourceServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer sourceServer.Close()

	destServer := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer destServer.Close()

	params := s.Param().(clipboardTestParams)

	path := "/text_1.html"
	// Update the policy blob.
	pb := policyBlob.NewBlob()
	switch params.restriction {
	case restrictionlevel.Blocked:
		pb.AddPolicies(policy.ClipboardBlockPolicy(sourceServer.URL+path, destServer.URL))
	case restrictionlevel.WarnCancelled, restrictionlevel.WarnProceeded:
		pb.AddPolicies(policy.ClipboardWarnPolicy(sourceServer.URL+path, destServer.URL))
	}

	// Update policy.
	if err := policyutil.ServeBlobAndRefresh(ctx, fakeDMS, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
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

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, params.browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	sourceURL := sourceServer.URL + path
	sourceConn, err := br.NewConn(ctx, sourceURL)
	if err != nil {
		s.Fatalf("Failed to open page %q: %v", path, err)
	}
	defer sourceConn.Close()

	if err := webutil.WaitForQuiescence(ctx, sourceConn, 10*time.Second); err != nil {
		s.Fatalf("Failed to wait for %q to achieve quiescence: %v", path, err)
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
	destConn, err := br.NewConn(ctx, destURL)
	if err != nil {
		s.Fatalf("Failed to open page %q: %v", destURL, err)
	}
	defer destConn.Close()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+params.name)

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
	parsedSourceURL, _ := url.Parse(sourceServer.URL)

	switch params.restriction {
	case restrictionlevel.Blocked:
		notifError := clipboard.CheckClipboardBubble(ctx, ui, parsedSourceURL.Hostname())
		if !params.copyAllowed && notifError != nil {
			s.Error("Expected notification but found an error: ", notifError)
		}
		if params.copyAllowed && notifError == nil {
			s.Error("Didn't expect notification but one was found: ")
		}
	case restrictionlevel.WarnCancelled:
		bubbleClass, notifError := clipboard.WarnBubble(ctx, ui, parsedSourceURL.Hostname())
		if notifError != nil {
			s.Error("Expected notification but found an error: ", notifError)
		}
		cancelButton := nodewith.Name("Cancel").Role(role.Button).Ancestor(bubbleClass)
		if err := ui.LeftClick(cancelButton)(ctx); err != nil {
			s.Fatal("Failed to click the cancel button: ", err)
		}
	case restrictionlevel.WarnProceeded:
		bubbleClass, notifError := clipboard.WarnBubble(ctx, ui, parsedSourceURL.Hostname())
		if notifError != nil {
			s.Error("Expected notification but found an error: ", notifError)
		}
		pasteButton := nodewith.Name("Paste anyway").Role(role.Button).Ancestor(bubbleClass)
		if err := ui.LeftClick(pasteButton)(ctx); err != nil {
			s.Fatal("Failed to click the paste button: ", err)
		}
	}

	if params.copyAllowed {
		pastedError := clipboard.CheckPastedContent(ctx, ui, copiedString)
		if pastedError != nil {
			s.Error("Checked pasted content but found an error: ", pastedError)
		}
	} else {
		emptyError := clipboard.CheckContentIsNotPasted(ctx, ui, copiedString)
		if emptyError != nil {
			s.Error("Content was pasted but should have been blocked")
		}
	}

}
