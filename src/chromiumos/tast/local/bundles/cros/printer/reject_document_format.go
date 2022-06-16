// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package printer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         RejectDocumentFormat,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Tests that lpadmin handles a printer rejecting get-printer-attributes requests containing the document-format attribute",
		Contacts:     []string{"project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"informational",
			"group:paper-io",
			"paper-io_printing",
		},
		Timeout:      2 * time.Minute,
		SoftwareDeps: []string{"chrome", "cros_internal", "virtual_usb_printer"},
		Data:         []string{"reject_document_format_script.textproto"},
		Fixture:      "virtualUsbPrinterModulesLoadedWithChromeLoggedIn",
	})
}

func RejectDocumentFormat(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	printer, err := usbprinter.Start(ctx,
		usbprinter.WithIPPUSBDescriptors(),
		usbprinter.WithGenericIPPAttributes(),
		usbprinter.WithMockPrinterScriptPath(s.DataPath("reject_document_format_script.textproto")))
	if err != nil {
		s.Fatal("Failed to start IPP-over-USB printer: ", err)
	}
	defer func(ctx context.Context) {
		if err := printer.Stop(ctx); err != nil {
			s.Error("Failed to stop printer: ", err)
		}
	}(ctx)

	uri := fmt.Sprintf("ippusb://%s_%s/ipp/print", printer.DevInfo.VID, printer.DevInfo.PID)

	cmd := testexec.CommandContext(ctx, "lpadmin", "-v", uri, "-m", "everywhere")
	_, stderr, err := cmd.SeparatedOutput()
	// testexec.ExitCode doesn't return the right value here, so we have to
	// search for the exit code ourselves.
	if err == nil || !strings.Contains(err.Error(), "exit status 9") {
		s.Fatal("Expected exit status 9 from `lpadmin`: ", err)
	}
	if !strings.Contains(string(stderr), "Failed to execute Get-Printer-Attributes request - retrying without document-format attribute") {
		s.Fatal("Expected error message for retrying without document-format: ", string(stderr))
	}
}
