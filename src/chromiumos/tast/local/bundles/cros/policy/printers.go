// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
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

	printerName := "Water Cooler Printer"
	printersPolicy := &policy.Printers{Val: []string{
		fmt.Sprintf(`{
			"display_name": "%s",
			"description": "The printer next to the water cooler.",
			"manufacturer": "Printer Manufacturer",
			"model": "Color Laser 2004",
			"uri": "ipps://print-server.intranet.example.com:443/ipp/cl2k4",
			"uuid": "1c395fdb-5d93-4904-b246-b2c046e79d12",
			"ppd_resource": {
				"effective_model":
				"Printer Manufacturer ColorLaser2k4",
				"autoconf": false
			}
		}`, printerName)}}

	if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
		s.Fatal("Failed to clean up: ", err)
	}

	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{printersPolicy}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	// TODO(crbug.com/1259615): This should be part of the fixture.
	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), browser.TypeLacros)
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Open an empty page. The print dialog fails to open when invoking CTRL+P
	// directly after calling `browserfixt.SetUp`, likely because the page isn't
	// fully loaded yet. In contrast to `browserfixt.SetUp`, `br.NewConn` waits
	// for the page to load.
	conn, err := br.NewConn(ctx, "chrome://newtab")
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()

	// Open print dialog
	if err := kb.Accel(ctx, "Ctrl+P"); err != nil {
		s.Fatal(errors.Wrap(err, "failed to type printing hotkey"))
	}
	ui := uiauto.New(tconn)
	if err := ui.WaitUntilExists(nodewith.Name("Print").ClassName("RootView").Role(role.Window))(ctx); err != nil {
		s.Fatal(errors.Wrap(err, "failed to check for existence of 'Print' window"))
	}

	if err := uiauto.Combine("check that the printer is available",
		ui.LeftClick(nodewith.Role("popUpButton").NameStartingWith("Destination")),
		ui.LeftClick(nodewith.Role("menuItem").Name("See more destinations")),
		ui.WaitUntilExists(nodewith.Role("cell").NameStartingWith(printerName)))(ctx); err != nil {
		s.Fatal(errors.Wrap(err, "failed to open destination popup and to check the existence of the printer"))
	}
}
