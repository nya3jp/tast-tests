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
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/printmanagementapp"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Printers,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of Printers policy, checking that configured printers are available to users in the printer selection after setting the policy",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
			"project-bolton@google.com",
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
			"informational",
		},
		Fixture: fixture.LacrosPolicyLoggedIn,
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.Printers{}, pci.VerifiedFunctionalityUI),
		},
	})
}

// Printers tests the Printers policy.
func Printers(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

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

	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{printersPolicy}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	// TODO(crbug.com/1259615): This should be part of the fixture.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browser.TypeLacros)
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Open a new tab. The print dialog fails to open when invoking CTRL+P
	// directly after calling `browserfixt.SetUp`, likely because the page
	// isn't fully loaded yet. It also fails to open on about:blank pages, but
	// works fine on chrome://newtab; see crbug.com/1290797.
	conn, err := br.NewConn(ctx, "chrome://newtab")
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_")

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get the keyboard: ", err)
	}
	defer kb.Close()

	ui := uiauto.New(tconn)
	if err := uiauto.Combine("select the printer and print",
		kb.AccelAction("Ctrl+P"),
		ui.WaitUntilExists(nodewith.Role(role.Window).Name("Print").ClassName("RootView")),
		ui.DoDefault(nodewith.Role(role.PopUpButton).NameStartingWith("Destination")),
		ui.DoDefault(nodewith.Role(role.MenuItem).Name("See more destinations")),
		ui.DoDefault(nodewith.Role(role.Cell).NameStartingWith(printerName)),
		ui.DoDefault(nodewith.Role(role.Button).Name("Print")),
		// Wait for the print preview window to close. Otherwise, the print preview
		// may still be in the process of closing when we launch the print
		// management app, and, once fully closed, steal focus from the print
		// management app.
		ui.WaitUntilGone(nodewith.Role(role.Window).Name("Print").ClassName("RootView")),
	)(ctx); err != nil {
		s.Fatal("Failed to select printer in print destination popup and print: ", err)
	}

	printManagementApp, err := printmanagementapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Print Management app: ", err)
	}
	if err := printManagementApp.VerifyPrintJob()(ctx); err != nil {
		s.Fatal("Failed to check existence of print job: ", err)
	}
}
