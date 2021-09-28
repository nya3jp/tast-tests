// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/restriction"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

// testFunc contains the contents of the test itself.
type testFunc func(ctx context.Context, tconn *chrome.TestConn) (bool, error)

func init() {
	testing.AddTest(&testing.Test{
		Func: PrintingEnabled,
		Desc: "Behavior of PrintingEnabled policy, checking the correspoding menu item restriction and printing preview dialog after setting the policy",
		Contacts: []string{
			"omse@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
		},
		Fixture: fixture.ChromePolicyLoggedIn,
		Params: []testing.Param{
			{
				Name: "print_from_chrome_menu",
				Val:  testPrintingFromThreeDotMenu,
			}, {
				Name:      "print_with_hotkey",
				Val:       testPrintingWithHotkey,
				ExtraAttr: []string{"informational"},
				Timeout:   3 * time.Minute,
			},
			{
				Name: "print_from_context_menu",
				Val:  testPrintingFromContextMenu,
			}},
	})
}

// PrintingEnabled tests the PrintingEnabled policy.
func PrintingEnabled(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	runTest := s.Param().(func(ctx context.Context, tconn *chrome.TestConn) (bool, error))

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name            string
		printingAllowed bool                    // printingAllowed indicates whether it should be possible to print the page.
		value           *policy.PrintingEnabled // value is the value of the policy.
	}{
		{
			name:            "unset",
			printingAllowed: true,
			value:           &policy.PrintingEnabled{Stat: policy.StatusUnset},
		},
		{
			name:            "enabled",
			printingAllowed: true,
			value:           &policy.PrintingEnabled{Val: true},
		},
		{
			name:            "disabled",
			printingAllowed: false,
			value:           &policy.PrintingEnabled{Val: false},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open an empty page in order to show Chrome UI.
			conn, err := cr.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to create an empty page: ", err)
			}
			defer conn.Close()

			// Make a call to the test case body.
			printingPossible, err := runTest(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to run test body: ", err)
			}

			if printingPossible != param.printingAllowed {
				s.Errorf("Unexpected printing restriction; got: %t, want: %t", printingPossible, param.printingAllowed)
			}
		})
	}
}

// testPrintingFromThreeDotMenu tests whether printing is possible via Chrome's dropdown menu.
func testPrintingFromThreeDotMenu(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	// Click the three dot button node.
	if err := uiauto.New(tconn).WithTimeout(10 * time.Second).LeftClick(nodewith.Role(role.PopUpButton).ClassName("BrowserAppMenuButton"))(ctx); err != nil {
		return false, errors.Wrap(err, "failed to click on dropdown menu")
	}

	printingPossible, err := checkPrintMenuItemIsRestricted(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "failed to check print menu item restriction")
	}

	return printingPossible, nil
}

// testPrintingWithHotkey tests whether printing is possible via hotkey (Ctrl + P).
func testPrintingWithHotkey(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	// Define keyboard to type keyboard shortcut.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		return false, errors.Wrap(err, "failed to get the keyboard")
	}
	defer kb.Close()

	// Type the shortcut.
	if err := kb.Accel(ctx, "Ctrl+P"); err != nil {
		return false, errors.Wrap(err, "failed to type printing hotkey")
	}

	// Check if printing dialog has appeared.
	printWindowExists := true
	ui := uiauto.New(tconn)
	finder := nodewith.Name("Print").ClassName("RootView").Role(role.Window)
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(finder)(ctx); err != nil {
		// If function above failed, it could be either a timeout or an actual error. Check once again.
		printWindowExists, err = ui.IsNodeFound(ctx, finder)
		// If the dialog does not exist by now, we assume that it will never be displayed.
		if err != nil {
			return false, errors.Wrap(err, "failed to check for printing windows existance")
		}
	}

	return printWindowExists, nil
}

// testPrintingFromContextMenu tests whether printing is possible via web page context menu.
func testPrintingFromContextMenu(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	// Find the webview node.
	ui := uiauto.New(tconn)
	webViewNode := nodewith.Role(role.WebView)
	if err := ui.WithTimeout(10 * time.Second).WaitUntilExists(webViewNode)(ctx); err != nil {
		return false, errors.Wrap(err, "failed to find web view")
	}

	// Invoke context menu of the web page.
	if err := ui.RightClick(webViewNode)(ctx); err != nil {
		return false, errors.Wrap(err, "failed to right click web view")
	}

	printingPossible, err := checkPrintMenuItemIsRestricted(ctx, tconn)
	if err != nil {
		return false, errors.Wrap(err, "failed to check print menu item restriction")
	}

	return printingPossible, nil
}

func checkPrintMenuItemIsRestricted(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
	// Find the printing menu item.
	ui := uiauto.New(tconn)
	menuItemNode := nodewith.Role(role.MenuItem).Name("Print… Ctrl+P")
	if err := ui.WithTimeout(5 * time.Second).WaitUntilExists(menuItemNode)(ctx); err != nil {
		return false, errors.Wrap(err, "failed to find print menu item")
	}

	// Check whether the printing menu item is restricted.
	info, err := ui.Info(ctx, menuItemNode)
	if err != nil {
		return false, errors.Wrap(err, "failed to get menuitem node infomation")
	}
	return info.Restriction != restriction.Disabled, nil
}
