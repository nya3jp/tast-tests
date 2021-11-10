// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package launcher

import (
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/launcher"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

type searchSettingsTestCase struct {
	searchTerm        string // searchTerm is the text that is entered in the Launcher.
	wantValue         string // wantValue is the expected string that should be present on the opened OS Settings section (e.g. a page title).
	passwordProtected bool   // whether the settings page is password protected.
}

const deviceUserPassword = "testpass"

func init() {
	testing.AddTest(&testing.Test{
		Func: SearchSettingsSections,
		Desc: "Launches OS Settings sections through the launcher",
		Contacts: []string{
			"anastasiian@chromium.org",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
		Params: []testing.Param{{
			Val: []searchSettingsTestCase{
				{
					searchTerm:        "Guest browsing",
					wantValue:         "Manage other people",
					passwordProtected: false,
				},
				{
					searchTerm:        "Show usernames and photos on the sign-in screen",
					wantValue:         "Manage other people",
					passwordProtected: false,
				},
				{
					searchTerm:        "Restrict sign-in",
					wantValue:         "Manage other people",
					passwordProtected: false,
				},
				{
					searchTerm:        "Add restricted user",
					wantValue:         "Manage other people",
					passwordProtected: false,
				},
				{
					searchTerm:        "Screen lock PIN",
					wantValue:         "Lock screen and sign-in",
					passwordProtected: true,
				},
				{
					searchTerm:        "Lock screen",
					wantValue:         "Lock screen and sign-in",
					passwordProtected: true,
				},
				{
					searchTerm:        "Add Google account",
					wantValue:         "Add Google account",
					passwordProtected: false,
				},
			},
		}, {
			Name:              "fingerprint_tests",
			ExtraHardwareDeps: hwdep.D(hwdep.Fingerprint()),
			Val: []searchSettingsTestCase{
				{
					searchTerm:        "Fingerprint settings",
					wantValue:         "Fingerprint",
					passwordProtected: true,
				},
				{
					searchTerm:        "Add fingerprint",
					wantValue:         "Fingerprint",
					passwordProtected: true,
				},
			},
		}},
	})
}

func SearchSettingsSections(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to find keyboard: ", err)
	}
	defer kb.Close()

	tcs := s.Param().([]searchSettingsTestCase)
	for _, tc := range tcs {
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

			ui := uiauto.New(tconn)
			searchResultView := nodewith.ClassName("SearchResultPageView")
			result := nodewith.NameStartingWith(tc.searchTerm).Ancestor(searchResultView).First()
			if err := uiauto.Combine("search for result in launcher",
				launcher.Open(tconn),
				launcher.Search(tconn, kb, tc.searchTerm),
				ui.WaitUntilExists(result),
				ui.LeftClick(result),
			)(ctx); err != nil {
				s.Fatalf("Failed to search for result %q in launcher: %v", tc.searchTerm, err)
			}

			if tc.passwordProtected {
				err := enterPassword(ctx, ui, kb, deviceUserPassword)
				if err != nil {
					s.Fatal("Failed to enter password: ", err)
				}
			}

			expectedNode := nodewith.Name(tc.wantValue).First()
			if err := ui.WaitUntilExists(expectedNode)(ctx); err != nil {
				s.Fatalf("Failed to find the node with name %q: %v", tc.wantValue, err)
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
