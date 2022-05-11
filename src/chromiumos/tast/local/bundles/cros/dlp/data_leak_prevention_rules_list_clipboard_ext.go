// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	policyBlob "chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/dlp/clipboard"
	"chromiumos/tast/local/bundles/cros/dlp/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListClipboardExt,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with clipboard blocked restriction when accessed by extension",
		Contacts: []string{
			"ayaelattar@google.com",
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"manifest.json", "background.js", "content.js"},
		Params: []testing.Param{{
			Name:    "ash",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
	})
}
func DataLeakPreventionRulesListClipboardExt(ctx context.Context, s *testing.State) {
	fakeDMS := s.FixtValue().(*fakedms.FakeDMS)

	// DLP policy with all clipboard blocked restriction.
	policyDLP := policy.RestrictiveDLPPolicyForClipboard()

	// Update the policy blob.
	pb := policyBlob.NewBlob()
	pb.AddPolicies(policyDLP)
	if err := fakeDMS.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	extDir, err := ioutil.TempDir("", "tast.dlp.Clipboard.")
	if err != nil {
		s.Fatal("Failed to create temp extension dir: ", err)
	}
	defer os.RemoveAll(extDir)

	extID, err := setUpExtension(ctx, s, extDir)
	if err != nil {
		s.Fatal("Failed setup of DLP Clipboard extension: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	// Policies are only updated after Chrome startup.
	cr, err := chrome.New(ctx,
		chrome.UnpackedExtension(extDir),
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fakeDMS.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	bgURL := chrome.ExtensionBackgroundPageURL(extID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		s.Fatalf("Failed to connect to background page at %v: %v", bgURL, err)
	}
	defer conn.Close()

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

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("No display: ", err)
	}

	// Display bounds.
	displayWidth := int(info.Bounds.Width)
	displayHeight := int(info.Bounds.Height)

	// Check extension access with restricted and non-restricted site.
	// See RestrictiveDLPPolicyForClipboard function in policy package for more details.
	for _, param := range []struct {
		name          string
		url           string
		accessAllowed bool
	}{
		{
			name:          "accessDenied",
			url:           "www.example.com",
			accessAllowed: false,
		},
		{
			name:          "accessAllowed",
			url:           "www.chromium.org",
			accessAllowed: true,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to open the browser: ", err)
			}
			defer closeBrowser(ctx)

			conn, err := br.NewConn(ctx, "https://"+param.url)
			if err != nil {
				s.Fatal("Failed to open page: ", err)
			}
			defer conn.Close()

			if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
				s.Fatalf("Failed to wait for %q to be loaded and achieve quiescence: %s", param.url, err)
			}

			ui := uiauto.New(tconn)
			if err := uiauto.Combine("copy all text from source website",
				keyboard.AccelAction("Ctrl+A"),
				keyboard.AccelAction("Ctrl+C"))(ctx); err != nil {
				s.Fatal("Failed to copy text from source browser: ", err)
			}

			googleConn, err := br.NewConn(ctx, "https://google.com")
			if err != nil {
				s.Fatal("Failed to open page: ", err)
			}
			defer googleConn.Close()

			if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
				s.Fatal("Failed to wait for google.com to be loaded and achieve quiescence: ", err)
			}

			if err := uiauto.Combine("Select tab and press Ctrl+Z",
				// Select tab for the extension.
				ui.MouseClickAtLocation(0, coords.Point{X: displayWidth / 2, Y: displayHeight / 2}),
				// A custom command to which DLP extension listens and then reads clipboard data.
				keyboard.AccelAction("Ctrl+Z"))(ctx); err != nil {
				s.Fatal("Failed to select tab and press Ctrl+Z: ", err)
			}

			expectedTitle := "Extension Restricted"
			if param.accessAllowed {
				expectedTitle = "Extension Access"
			}
			var actualTitle string

			// This can be too fast, so poll till the extension updates the webpage title.
			if err := testing.Poll(ctx, func(ctx context.Context) error {
				if err := googleConn.Eval(ctx, "document.title", &actualTitle); err != nil {
					return errors.Wrap(err, "failed to get the webpage title")
				}

				if expectedTitle != actualTitle {
					return errors.New("Page title not as expected")
				}

				return nil
			}, &testing.PollOptions{
				Timeout:  5 * time.Second,
				Interval: 1 * time.Second,
			}); err != nil {
				s.Fatalf("Found page title %s, expected %s: %s", actualTitle, expectedTitle, err)
			}

			err = clipboard.CheckClipboardBubble(ctx, ui, param.url)
			// Clipboard DLP bubble is not expected when access allowed.
			if err == nil && param.accessAllowed {
				s.Error("Notification found, expected none")
			}

			// Clipboard DLP bubble is expected when access not allowed.
			if err != nil && !param.accessAllowed {
				s.Error("Notification not found, expected DLP clipboard notification")
			}
		})
	}
}

// setUpExtension moves the extension files into the extension directory and returns extension ID.
func setUpExtension(ctx context.Context, s *testing.State, extDir string) (string, error) {
	for _, name := range []string{"manifest.json", "background.js", "content.js"} {
		if err := fsutil.CopyFile(s.DataPath(name), filepath.Join(extDir, name)); err != nil {
			return "", errors.Wrapf(err, "failed to copy file %s", name)
		}
	}

	extID, err := chrome.ComputeExtensionID(extDir)
	if err != nil {
		s.Fatalf("Failed to compute extension ID for %v: %v", extDir, err)
	}

	return extID, nil
}
