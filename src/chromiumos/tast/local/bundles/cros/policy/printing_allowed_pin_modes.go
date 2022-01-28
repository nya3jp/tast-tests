// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
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
	"chromiumos/tast/local/chrome/uiauto/printpreview"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/printing/printer"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

type printingAllowedPinModeTestParams struct {
	browserType browser.Type
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         PrintingAllowedPinModes,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Behavior of default pin print policy, checking the correspoding ui restriction and printing preview dialog after setting the policy",
		Contacts: []string{
			"abuaboud@google.com", // Test author
			"chromeos-commercial-networking@google.com",
		},
		SoftwareDeps: []string{"chrome", "cros_internal", "cups", "virtual_usb_printer"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_printing",
			"informational",
		},
		Params: []testing.Param{
			{
				Name:      "pin_printing_default_policy",
				Fixture:   fixture.ChromePolicyLoggedIn,
				Val:       printingAllowedPinModeTestParams{browserType: browser.TypeAsh},
				ExtraAttr: []string{"informational"},
				Timeout:   3 * time.Minute,
			}},
	})
}

func PrintingAllowedPinModes(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	data := s.Param().(printingAllowedPinModeTestParams)

	// Connect to Test API to use it with the UI library.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	for _, param := range []struct {
		name  string
		value *policy.PrintingAllowedPinModes // value is the value of the policy.
	}{
		{
			name:  "pin",
			value: &policy.PrintingAllowedPinModes{Val: "pin"},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), data.browserType)
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			// Open an empty page in order to show Chrome UI.
			conn, err := br.NewConn(ctx, "")
			if err != nil {
				s.Fatal("Failed to create an empty page: ", err)
			}
			defer conn.Close()

			addUSBPrinter(ctx, s)

			printWindowExists, err := openPrintPreviewWithHotkey(ctx, tconn)
			if printWindowExists == false {
				s.Fatal("Failed to open print preview: ", err)
			}
			s.Log("Waiting for print preview to load")
			if err := printpreview.WaitForPrintPreview(tconn)(ctx); err != nil {
				s.Fatal("Failed to wait for Print Preview: ", err)
			}

		})
	}
}

func addUSBPrinter(ctx context.Context, s *testing.State) {
	if err := printer.ResetCups(ctx); err != nil {
		s.Fatal("Failed to reset cupsd: ", err)
	}
	printer, err := usbprinter.Start(ctx,
		usbprinter.WithIPPUSBDescriptors(),
		usbprinter.WithGenericIPPAttributes())
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func(ctx context.Context) {
		if err := printer.Stop(ctx); err != nil {
			s.Error("Failed to stop virtual printer: ", err)
		}
	}(ctx)
}

func openPrintPreviewWithHotkey(ctx context.Context, tconn *chrome.TestConn) (bool, error) {
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
		// If the dialog does not exist by now, we assume that it will never be displayed.
		if err != nil {
			return false, errors.Wrap(err, "failed to check for printing windows existance")
		}
	}
	return printWindowExists, nil
}
