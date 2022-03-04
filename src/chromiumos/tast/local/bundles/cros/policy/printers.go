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
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/printmanagementapp"
	"chromiumos/tast/local/chrome/uiauto/printpreview"
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
		SoftwareDeps: []string{"chrome"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
			"informational",
		},
		Params: []testing.Param{
			{
				Fixture: fixture.ChromePolicyLoggedIn,
				Val:     browser.TypeAsh,
			}, {
				Name:              "lacros",
				Fixture:           fixture.LacrosPolicyLoggedIn,
				Val:               browser.TypeLacros,
				ExtraSoftwareDeps: []string{"lacros"},
			},
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
	printerDescription := "The printer next to the water cooler"
	printersPolicy := &policy.Printers{Val: []string{
		fmt.Sprintf(`{
			"display_name": "%s",
			"description": "%s",
			"manufacturer": "Printer Manufacturer",
			"model": "Color Laser 2004",
			"uri": "lpd://localhost:9100",
			"uuid": "1c395fdb-5d93-4904-b246-b2c046e79d12",
			"ppd_resource": {
				"effective_model": "generic pcl 6/pcl xl printer pxlcolor",
				"autoconf": false
			}
		}`, printerName, printerDescription)}}

	if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{printersPolicy}); err != nil {
		s.Fatal("Failed to update policies: ", err)
	}

	// TODO(crbug.com/1259615): This should be part of the fixture.
	conn, _, closeBrowser, err := browserfixt.SetUpWithURL(ctx, s.FixtValue(), s.Param().(browser.Type), "")
	if err != nil {
		s.Fatal("Failed to setup chrome: ", err)
	}
	defer closeBrowser(cleanupCtx)
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

	if err := uiauto.Combine("open print preview",
		kb.AccelAction("Ctrl+P"),
		printpreview.WaitForPrintPreview(tconn),
	)(ctx); err != nil {
		s.Fatal("Failed to open print preview: ", err)
	}

	if err := printpreview.SelectPrinter(ctx, tconn, fmt.Sprintf("%s %s", printerName, printerDescription)); err != nil {
		s.Fatal("Failed to select printer: ", err)
	}

	if err := printpreview.Print(ctx, tconn); err != nil {
		s.Fatal("Failed to print: ", err)
	}

	printManagementApp, err := printmanagementapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch Print Management app: ", err)
	}
	if err := printManagementApp.VerifyPrintJob()(ctx); err != nil {
		s.Fatal("Failed to check existence of print job: ", err)
	}
}
