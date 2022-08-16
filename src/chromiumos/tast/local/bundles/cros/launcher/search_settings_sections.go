// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type searchSettingsTestCase struct {
	searchTerm        string           // searchTerm is the text that is entered in the Launcher.
	searchResult      string           // searchResult is the text that should be selected in the Launcher.
	wantValue         *nodewith.Finder // wantValue is the expected node that should be present on the opened OS Settings section (e.g. a page title).
	passwordProtected bool             // whether the settings page is password protected.
}

type searchSettingsVals struct {
	tabletMode bool                     // whether the device is on tablet mode.
	testCases  []searchSettingsTestCase // test cases
}

const deviceUserPassword = "testpass"
const settingsWindowTitle = "Settings"

var standardTestCases = []searchSettingsTestCase{
	{
		searchTerm:        "Guest browsing",
		searchResult:      "Guest browsing, Manage other people",
		wantValue:         nodewith.Name("Manage other people").Role(role.Heading),
		passwordProtected: false,
	},
	{
		searchTerm:        "Show usernames and photos on the sign-in screen",
		searchResult:      "Show usernames and photos on the sign-in screen, Manage other people",
		wantValue:         nodewith.Name("Manage other people").Role(role.Heading),
		passwordProtected: false,
	},
	{
		searchTerm:        "Restrict sign-in",
		searchResult:      "Restrict sign-in, Manage other people",
		wantValue:         nodewith.Name("Manage other people").Role(role.Heading),
		passwordProtected: false,
	},
	{
		searchTerm:        "Add restricted user",
		searchResult:      "Add restricted user, Manage other people",
		wantValue:         nodewith.Name("Manage other people").Role(role.Heading),
		passwordProtected: false,
	},
	{
		searchTerm:        "Screen lock PIN",
		searchResult:      "Screen lock PIN, Lock screen and sign-in",
		wantValue:         nodewith.NameStartingWith("Lock screen").Role(role.Heading),
		passwordProtected: true,
	},
	{
		searchTerm:        "Lock screen",
		searchResult:      "Lock screen and sign-in, Security and Privacy",
		wantValue:         nodewith.NameStartingWith("Lock screen").Role(role.Heading),
		passwordProtected: true,
	},
	{
		searchTerm:        "Add Google account",
		searchResult:      "Add Google Account, My accounts",
		wantValue:         nodewith.Name("Add Google Account").Role(role.Button),
		passwordProtected: false,
	},
}

var fingerprintTestCases = []searchSettingsTestCase{
	{
		searchTerm:        "Fingerprint settings",
		searchResult:      "Fingerprint settings, Lock screen and sign-in",
		wantValue:         nodewith.Name("Fingerprint").Role(role.Heading),
		passwordProtected: true,
	},
	{
		searchTerm:        "Add fingerprint",
		searchResult:      "Add fingerprint, Fingerprint",
		wantValue:         nodewith.Name("Fingerprint").Role(role.Heading),
		passwordProtected: true,
	},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         SearchSettingsSections,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Searches for sections in OS Settings using Launcher search, and checks that the correct pages are opened",
		Contacts: []string{
			"anastasiian@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithProductivityLauncher",
		Params: []testing.Param{{
			Name:      "clamshel_mode",
			ExtraAttr: []string{"informational"},
			Val:       searchSettingsVals{tabletMode: false, testCases: standardTestCases},
		}, {
			Name:              "tablet_mode",
			ExtraAttr:         []string{"informational"},
			Val:               searchSettingsVals{tabletMode: true, testCases: standardTestCases},
			ExtraHardwareDeps: hwdep.D(hwdep.InternalDisplay()),
		}, {
			Name:              "fingerprint_tests_clamshell_mode",
			ExtraHardwareDeps: hwdep.D(hwdep.Fingerprint()),
			Val:               searchSettingsVals{tabletMode: false, testCases: fingerprintTestCases},
		}, {
			Name:              "fingerprint_tests_tablet_mode",
			ExtraHardwareDeps: hwdep.D(hwdep.Fingerprint(), hwdep.InternalDisplay()),
			Val:               searchSettingsVals{tabletMode: true, testCases: fingerprintTestCases},
		}},
	})
}

func SearchSettingsSections(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	cleanupCtx := ctx

	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	testParams := s.Param().(searchSettingsVals)
	cleanup, err := launcher.SetUpLauncherTest(ctx, tconn, testParams.tabletMode, true /*productivityLauncher*/, false /*stabilizeAppCount*/)
	if err != nil {
		s.Fatal("Failed to set up launcher test case: ", err)
	}
	defer cleanup(cleanupCtx)

	for _, tc := range testParams.testCases {
		s.Run(ctx, tc.searchTerm, func(ctx context.Context, s *testing.State) {
			defer func(ctx context.Context) {
				// Cleanup: close the OS Settings window.
				activeWindow, err := ash.GetActiveWindow(ctx, tconn)
				if err != nil {
					s.Fatal("Failed to get the active window: ", err)
				}
				if err := activeWindow.CloseWindow(ctx, tconn); err != nil {
					s.Fatalf("Failed to close the window(%s): %v", activeWindow.Name, err)
				}
			}(ctx)

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+tc.searchTerm)

			ui := uiauto.New(tconn)
			result := launcher.SearchResultListItemFinder.NameStartingWith(tc.searchResult).First()
			if err := uiauto.Combine("search for result in launcher",
				launcher.Open(tconn),
				launcher.Search(tconn, kb, tc.searchTerm),
				ui.WaitUntilExists(result),
				ui.LeftClick(result),
			)(ctx); err != nil {
				s.Fatalf("Failed to search for result %q in launcher: %v", tc.searchTerm, err)
			}

			activeWindow, err := ash.GetActiveWindow(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get the active window: ", err)
			}
			if activeWindow.Title != settingsWindowTitle {
				s.Fatalf("Active window is %q, expected %q", activeWindow.Title, settingsWindowTitle)
			}

			if tc.passwordProtected {
				err := enterPassword(ctx, ui, kb, deviceUserPassword)
				if err != nil {
					s.Fatal("Failed to enter password: ", err)
				}
			}

			settings := nodewith.NameStartingWith("Settings").Role(role.Window).First()
			expectedNode := tc.wantValue.Ancestor(settings).First()
			if err := ui.WaitUntilExists(expectedNode)(ctx); err != nil {
				s.Fatalf("Failed to find the node %q: %v", tc.wantValue.Pretty(), err)
			}
		})
	}
}

func enterPassword(ctx context.Context, ui *uiauto.Context, kb *input.KeyboardEventWriter, password string) error {
	if err := ui.WaitUntilExists(nodewith.Name("Confirm your password").First())(ctx); err != nil {
		return errors.Wrap(err, "could not find password dialog")
	}

	if err := kb.Type(ctx, password+"\n"); err != nil {
		return errors.Wrap(err, "failed to type password")
	}

	return nil
}
