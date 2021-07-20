// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/bundles/cros/dlp/clipboard"
	"chromiumos/tast/local/bundles/cros/dlp/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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
		Data:         []string{"manifest.json", "background.js", "dlp_clipboard.html", "dlp_clipboard.js"},
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
	// // Update policy.
	// if err := policyutil.ServeBlobAndRefresh(ctx, fakeDMS, cr, pb); err != nil {
	// 	s.Fatal("Failed to serve and refresh: ", err)
	// }

	extDir, err := ioutil.TempDir("", "tast.documentscanapi.Scan.")
	if err != nil {
		s.Fatal("Failed to create temp extension dir: ", err)
	}
	defer os.RemoveAll(extDir)

	extID, err := setUpDocumentScanExtension(ctx, s, extDir)
	if err != nil {
		s.Fatal("Failed setup of Document Scan extension: ", err)
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

	fgURL := "chrome-extension://" + extID + "/dlp_clipboard.html"
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(fgURL))
	if err != nil {
		s.Fatalf("Could not connect to extension at %v: %v", fgURL, err)
	}
	defer conn.Close()

	for _, param := range []struct {
		name        string
		url         string
		wantAllowed bool
	}{
		{
			name:        "Example",
			url:         "www.example.com",
			wantAllowed: false,
		},
		{
			name:        "Saleseforce",
			url:         "www.salesforce.com",
			wantAllowed: true,
		},
		{
			name:        "Company",
			url:         "www.company.com",
			wantAllowed: false,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)
			if _, err = cr.NewConn(ctx, "https://"+param.url); err != nil {
				s.Error("Failed to open page: ", err)
			}
			if err := keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+C"); err != nil {
				s.Fatal("Failed to press Ctrl+C to copy content: ", err)
			}

			s.Log("Clicking Extension button")
			ui := uiauto.New(tconn)
			scanButton := nodewith.NameStartingWith("Extension ").Role(role.Button)

			if err := ui.LeftClick(scanButton)(ctx); err != nil {
				s.Fatal("Failed finding notification and clicking it: ", err)
			}

			s.Log("Checking copied content using extension")
			err = checkExtensionAccess(ctx, tconn, param.url, param.wantAllowed)
			if err != nil {
				s.Fatal("Failed to check copied content: ", err)
			}
			s.Log("Opening files app")
			err = openFilesApp(ctx, tconn, param.url)
			if err != nil {
				s.Fatal("Failed to open filesapp: ", err)
			}

		})
	}
}

// setUpDocumentScanExtension moves the extension files into the extension directory and returns extension ID.
func setUpDocumentScanExtension(ctx context.Context, s *testing.State, extDir string) (string, error) {
	for _, name := range []string{"manifest.json", "background.js", "dlp_clipboard.html", "dlp_clipboard.js"} {
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

func checkExtensionAccess(ctx context.Context, tconn *chrome.TestConn, url string, wantAllowed bool) error {
	ui := uiauto.New(tconn)

	accessButton := nodewith.Name("Extension able to access content").Role(role.Button)

	if err := ui.WaitUntilExists(accessButton)(ctx); err != nil && wantAllowed {
		return errors.Wrap(err, "failed to open files app: ")
	}

	noAccessButton := nodewith.Name("Extension couldn't access content").Role(role.Button)

	if err := ui.WaitUntilExists(noAccessButton)(ctx); err != nil && !wantAllowed {
		return errors.Wrap(err, "failed to open files app: ")
	}

	err := clipboard.CheckClipboardBubble(ctx, ui, url)
	if err == nil {
		return errors.New("Notification found, expected none")
	}
	return nil
}

func openFilesApp(ctx context.Context, tconn *chrome.TestConn, url string) error {
	ui := uiauto.New(tconn)
	// Open Files app
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open files app: ")
	}
	if err := filesApp.OpenDownloads()(ctx); err != nil {
		return errors.Wrap(err, "failed to open downloads: ")
	}
	err = clipboard.CheckClipboardBubble(ctx, ui, url)
	if err == nil {
		return errors.New("Notification found, expected none")
	}
	filesApp.Close(ctx)
	return nil
}
