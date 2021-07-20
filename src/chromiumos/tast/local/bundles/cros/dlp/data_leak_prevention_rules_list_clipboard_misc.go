// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/dlp/clipboard"
	"chromiumos/tast/local/bundles/cros/dlp/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataLeakPreventionRulesListClipboardMisc,
		Desc: "Test behavior of DataLeakPreventionRulesList policy with clipboard blocked restriction in miscellaneous conditions",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "fakeDMS",
		Data:         []string{"manifest.json", "background.js", "content.js"},
	})
}
func DataLeakPreventionRulesListClipboardMisc(ctx context.Context, s *testing.State) {
	fakeDMS := s.FixtValue().(*fakedms.FakeDMS)

	// DLP policy with all clipboard blocked restriction.
	policyDLP := policy.RestrictiveDLPPolicyForClipboard()

	// Update the policy blob.
	pb := fakedms.NewPolicyBlob()
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

	s.Log("Connecting to background page")
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
		s.Fatal("chrome.clipboard API unavailable: ", err)
	}

	info, err := display.GetInternalInfo(ctx, tconn)
	if err != nil {
		s.Fatal("No display: ", err)
	}

	// Display bounds
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
			name:          "example",
			url:           "www.example.com",
			accessAllowed: false,
		},
		{
			name:          "chromium",
			url:           "www.chromium.org",
			accessAllowed: true,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)
			conn, err := cr.NewConn(ctx, "https://"+param.url)
			if err != nil {
				s.Error("Failed to open page: ", err)
			}

			defer conn.Close()

			ui := uiauto.New(tconn)
			if err := keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+C"); err != nil {
				s.Fatal("Failed to press Ctrl+C to copy content: ", err)
			}

			googleConn, err := cr.NewConn(ctx, "https://google.com")
			if err != nil {
				s.Error("Failed to open page: ", err)
			}
			defer googleConn.Close()

			// Select the tab for extension.
			if err := ui.MouseClickAtLocation(0, coords.Point{X: displayWidth / 2, Y: displayHeight / 2})(ctx); err != nil {
				s.Fatal("Failed to select tab: ", err)
			}

			// A Custom command in extension 'DLP extension to get clipboard data' which doesn't affect clipboard content.
			if err := keyboard.Accel(ctx, "Ctrl+Z"); err != nil {
				s.Fatal("Failed to press Ctrl+Z to execute extension custom command: ", err)
			}

			err = checkExtensionAccess(ctx, tconn, param.url, param.accessAllowed)
			if err != nil {
				s.Error("Failed to check if extension can access data: ", err)
			}
		})
	}
}

// setUpExtension moves the extension files into the extension directory and returns extension ID.
func setUpExtension(ctx context.Context, s *testing.State, extDir string) (string, error) {
	for _, name := range []string{"manifest.json", "background.js", "content.js"} {
		if err := fsutil.CopyFile(s.DataPath(name), filepath.Join(extDir, name)); err != nil {
			return "", errors.Wrapf(err, "failed to copy file %q: %v", name, err)
		}
	}

	extID, err := chrome.ComputeExtensionID(extDir)
	if err != nil {
		s.Fatalf("Failed to compute extension ID for %q: %v", extDir, err)
	}

	return extID, nil
}

// checkExtensionAccess checks if extension can or cannot access data based upon it's restriction and doesn't show toast in all case.
func checkExtensionAccess(ctx context.Context, tconn *chrome.TestConn, url string, accessAllowed bool) error {
	wantTitle := "Extension Restricted"
	if accessAllowed {
		wantTitle = "Extension Access"
	}

	if err := tconn.WaitForExpr(ctx, fmt.Sprintf(`document.title === "%s"`, wantTitle)); err != nil {
		return errors.Wrap(err, "failed to check title of page")
	}

	ui := uiauto.New(tconn)
	// Clipboard DLP bubble is never expected when try to access it.
	err := clipboard.CheckClipboardBubble(ctx, ui, url)
	if err == nil {
		return errors.New("Notification found, want none")
	}
	return nil
}
