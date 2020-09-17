// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/pre"
	"chromiumos/tast/testing"
)

// testCase contains the function used in the test.
type testCase struct {
	TestFunc testFuncType
}

// testFuncType contains the contents of the test itself.
type testFuncType func(ctx context.Context, s *testing.State, tconn *chrome.TestConn, printingAllowed bool)

func init() {
	testing.AddTest(&testing.Test{
		Func: PrintingEnabled,
		Desc: "Behavior of PrintingEnabled policy, checking the correspoding menu item restriction and printing preview dialog after setting the policy",
		Contacts: []string{
			"omse@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Pre:          pre.User,
		Params: []testing.Param{
			{
				Name: "print_from_chrome_menu",
				Val: testCase{
					TestFunc: testPrintingFromThreeDotMenu,
				},
			}, {
				Name: "print_with_hotkey",
				Val: testCase{
					TestFunc: testPrintingWithHotkey,
				},
			},
			{
				Name: "print_from_context_menu",
				Val: testCase{
					TestFunc: testPrintingFromContextMenu,
				},
			}},
	})
}

// PrintingEnabled tests the PrintingEnabled policy.
func PrintingEnabled(ctx context.Context, s *testing.State) {
	cr := s.PreValue().(*pre.PreData).Chrome
	fdms := s.PreValue().(*pre.PreData).FakeDMS
	testParams := s.Param().(testCase)

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
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndRefresh(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Connect to Test API to use it with the ui library.
			tconn, err := cr.TestAPIConn(ctx)
			if err != nil {
				s.Fatal("Failed to create Test API connection: ", err)
			}

			// Open an empty page.
			conn, err := cr.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to create an empty page: ", err)
			}
			defer conn.Close()

			// Make a call to the test case body.
			testParams.TestFunc(ctx, s, tconn, param.printingAllowed)
		})
	}
}

// testPrintingFromThreeDotMenu tests whether printing is possible via Chrome's dropdown menu.
func testPrintingFromThreeDotMenu(ctx context.Context, s *testing.State, tconn *chrome.TestConn, printingAllowed bool) {
	// Find the three dot button node.
	menuNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Role:      ui.RoleTypePopUpButton,
		ClassName: "BrowserAppMenuButton",
	}, 10*time.Second)
	if err != nil {
		s.Fatal("Finding dropdown menu failed: ", err)
	}
	defer menuNode.Release(ctx)
	if err := menuNode.LeftClick(ctx); err != nil {
		s.Fatal("Performing click on dropdown menu failed: ", err)
	}

	// Find the printing menu item.
	menuItemNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Role: ui.RoleTypeMenuItem,
		Name: "Print… Ctrl+P",
	}, 5*time.Second)
	if err != nil {
		s.Fatal("Finding print menu item failed: ", err)
	}
	defer menuItemNode.Release(ctx)

	// Check whether the printing menu item is restricted.
	printingPossible := menuItemNode.Restriction != ui.RestrictionDisabled
	if printingPossible != printingAllowed {
		if printingAllowed == true {
			s.Error("Printing is allowed, but printing menu item is restricted")
		} else {
			s.Error("Printing is disallowed, but printing menu item is not restricted")
		}
	}
}

// testPrintingWithHotkey tests whether printing is possible via hotkey (Ctrl + P).
func testPrintingWithHotkey(ctx context.Context, s *testing.State, tconn *chrome.TestConn, printingAllowed bool) {
	// Define keyboard to type keyboard shortcut.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	// Type the shortcut.
	if err := kb.Accel(ctx, "Ctrl+P"); err != nil {
		s.Fatal("Failed to type printing hotkey: ", err)
	}

	// Check if printing dialog has appeared.
	printWindowExists := true
	if err := ui.WaitUntilExists(ctx, tconn, ui.FindParams{Name: "Print"}, 10*time.Second); err != nil {
		// If function above failed, it could be either a timeout or an actual error. Check once again.
		printWindowExists, err = ui.Exists(ctx, tconn, ui.FindParams{Name: "Print"})
		// If the dialog does not exist by now, we assume that it will never be displayed.
		if err != nil {
			s.Fatal("Could not check for printing windows existance: ", err)
		}
	}

	// Check whether the printing preview dialog has been displayed.
	if printWindowExists != printingAllowed {
		if printingAllowed == true {
			s.Error("Printing is allowed, but printing dialog has not been shown on a hotkey")
		} else {
			s.Error("Printing is disallowed, but printing dialog has been shown on a hotkey")
		}
	}
}

// testPrintingFromContextMenu tests whether printing is possible via web page context menu.
func testPrintingFromContextMenu(ctx context.Context, s *testing.State, tconn *chrome.TestConn, printingAllowed bool) {
	// Find the webview node.
	webViewNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Role: ui.RoleTypeWebView,
	}, 10*time.Second)
	if err != nil {
		s.Fatal("Finding web view failed: ", err)
	}
	defer webViewNode.Release(ctx)

	// Invoke context menu of the web page.
	if err := webViewNode.RightClick(ctx); err != nil {
		s.Fatal("Could not right click web view: ", err)
	}

	// Find the printing menu item.
	menuItemNode, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{
		Role: ui.RoleTypeMenuItem,
		Name: "Print… Ctrl+P",
	}, 5*time.Second)
	if err != nil {
		s.Fatal("Finding print menu item failed: ", err)
	}
	defer menuItemNode.Release(ctx)

	// Check whether the printing menu item is restricted.
	printingPossible := menuItemNode.Restriction != ui.RestrictionDisabled
	if printingPossible != printingAllowed {
		if printingAllowed == true {
			s.Error("Printing is allowed, but printing menu item is restricted")
		} else {
			s.Error("Printing is disallowed, but printing menu item is not restricted")
		}
	}
}
