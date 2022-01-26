// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
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
		Func:         Printers,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of Printers policy, checking that configured printers are available to users in the printer selection after setting the policy",
		Contacts: []string{
			"cmfcmf@google.com", // Test author
			"project-bolton@google.com",
			"chromeos-commercial-remote-management@google.com",
		},
		SoftwareDeps: []string{"chrome", "lacros"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
			"informational",
		},
		Fixture: fixture.LacrosPolicyLoggedIn,
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
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), browser.TypeLacros)
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
		s.Fatal(errors.Wrap(err, "failed to get the keyboard"))
	}
	defer kb.Close()

	ui := uiauto.New(tconn)
	if err := uiauto.Combine("select the printer and print",
		kb.AccelAction("Ctrl+P"),
		ui.WaitUntilExists(nodewith.Role(role.Window).Name("Print").ClassName("RootView")),
		ui.LeftClick(nodewith.Role(role.PopUpButton).NameStartingWith("Destination")),
		ui.LeftClick(nodewith.Role(role.MenuItem).Name("See more destinations")),
		ui.LeftClick(nodewith.Role(role.Cell).NameStartingWith(printerName)),
		ui.LeftClick(nodewith.Role(role.Button).Name("Print")),
	)(ctx); err != nil {
		s.Fatal(errors.Wrap(err, "failed to select printer in print destination popup and print"))
	}

	if err := uiauto.Combine("check that the print job is visible in print jobs overview",
		// Open Settings app
		kb.AccelAction("Search"),
		ui.WaitUntilExists(nodewith.Role(role.TextField).NameStartingWith("Search your device")),
		kb.TypeAction("Settings"),
		ui.LeftClick(nodewith.Role(role.ListBoxOption).Name("Settings, Installed App")),
		// Open "Print and scan" settings page
		ui.WaitUntilExists(nodewith.Role(role.SearchBox).Name("Search settings")),
		kb.TypeAction("Print jobs"),
		ui.WaitUntilExists(nodewith.Focusable().NameContaining("Print jobs. Press Enter to navigate to section.")),
		kb.AccelAction("Enter"),
		// Open "Print jobs" window
		ui.LeftClick(nodewith.Role(role.Link).Name("Print jobs View and manage print jobs")),
		ui.WaitUntilExists(nodewith.Role(role.Window).Name("Print jobs").ClassName("BrowserFrame")),
		// Verify that a print job for our printer exists and is not cancelled. The
		// Name contains the time and title of the page we printed, which we exclude
		// by using a Regexp.
		ui.WaitUntilExists(nodewith.Focusable().NameRegex(
			regexp.MustCompile(fmt.Sprintf(".*%s.*%s.*",
				regexp.QuoteMeta(printerName),
				regexp.QuoteMeta("Press enter to cancel the print job."))))),
	)(ctx); err != nil {
		s.Fatal(errors.Wrap(err, "failed to check existence of print job"))
	}
}
