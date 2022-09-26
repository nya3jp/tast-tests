// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
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
		Func:         DeletePrintJobHistoryAllowed,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Behavior of DeletePrintJobHistoryAllowed policy, checking the corresponding button state after setting the policy",
		Contacts: []string{
			"poromov@chromium.org", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      fixture.ChromePolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.DeletePrintJobHistoryAllowed{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// DeletePrintJobHistoryAllowed tests the DeletePrintJobHistoryAllowed policy.
func DeletePrintJobHistoryAllowed(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	uia := uiauto.New(tconn)

	// Get clipboard to print.
	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	for _, param := range []struct {
		name    string
		enabled bool                                 // enabled is the expected enabled state of the clear button.
		value   *policy.DeletePrintJobHistoryAllowed // value is the value of the policy.
	}{
		{
			name:    "deny",
			enabled: false,
			value:   &policy.DeletePrintJobHistoryAllowed{Val: false},
		},
		{
			name:    "allow",
			enabled: true,
			value:   &policy.DeletePrintJobHistoryAllowed{Val: true},
		},
		{
			name:    "unset",
			enabled: true,
			value:   &policy.DeletePrintJobHistoryAllowed{Stat: policy.StatusUnset},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Configure printer policy.
			printerName := "Water Cooler Printer"
			printersPolicy := &policy.Printers{Val: []string{
				fmt.Sprintf(`{
			  "display_name": "%s",
			  "description": "The printer next to the water cooler.",
			  "manufacturer": "Printer Manufacturer",
			  "model": "Color Laser 2004",
			  "uri": "lpd://localhost:9100",
			  "uuid": "1c395fdb-5d93-4904-b246-b2c046e79d12",
			  "ppd_resource": {
    			"effective_model": "generic pcl 6/pcl xl printer pxlcolor",
		  	"autoconf": false
			  }
		  }`, printerName)}}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value, printersPolicy}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// Open print management page.
			conn, err := cr.NewConn(ctx, "chrome://print-management")
			if err != nil {
				s.Fatal("Failed to connect to the print management page: ", err)
			}
			defer conn.Close()
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Print the current page.
			if err := uiauto.Combine("Select the printer and print",
				kb.AccelAction("Ctrl+P"),
				uia.WaitUntilExists(nodewith.Role(role.Window).Name("Print").ClassName("RootView")),
				uia.LeftClick(nodewith.Role(role.PopUpButton).NameStartingWith("Destination")),
				uia.LeftClick(nodewith.Role(role.MenuItem).Name("See more destinations")),
				uia.LeftClick(nodewith.Role(role.Cell).NameStartingWith(printerName)),
				uia.LeftClick(nodewith.Role(role.Button).Name("Print")),
			)(ctx); err != nil {
				s.Fatal("Failed to select printer in print destination popup and print: ", err)
			}

			// Cancel the print job.
			if err := uiauto.Combine("Cancel the print job",
				uia.FocusAndWait(nodewith.Role(role.Button).Ancestor(nodewith.NameContaining("Press enter to cancel the print job").First())),
				kb.AccelAction("Enter"),
			)(ctx); err != nil {
				s.Fatal("Failed to cancel the print job: ", err)
			}

			// Check whether clear button exists and is enabled.
			clearButton := nodewith.Name("Clear all history").Role(role.Button)
			clearButtonEnabled := nodewith.ClassName("delete-enabled")
			enabled := uiauto.Combine("Check clear button activity ",
				uia.WaitUntilExists(clearButton),
				uia.WaitUntilExists(clearButtonEnabled))(ctx) == nil

			if enabled != param.enabled {
				s.Errorf("Unexpected existence of print history clear button found: got %t; want %t", enabled, param.enabled)
			}
		})
	}
}
