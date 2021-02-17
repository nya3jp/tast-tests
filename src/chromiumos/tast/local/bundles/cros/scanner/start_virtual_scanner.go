// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanner

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/scanner/cups"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         StartVirtualScanner,
		Desc:         "Creates a virtual scanner device and leaves it running.",
		Contacts:     []string{"bmgordon@chromium.org", "project-bolton@google.com"},
		Attr:         []string{ /* None because this test should be run manually. */ },
		SoftwareDeps: []string{"virtual_usb_printer", "cups", "chrome"},
		Pre:          chrome.LoggedIn(),
		Data:         []string{sourceImage, goldenImage},
		Vars: []string{
			"descriptors", // USB descriptors
			"attributes",  // IPP attributes
			"escl_caps",   // eSCL capabilities
		},
	})
}

// deviceVars represents the variables that can be passed from the command line.
type deviceVars struct {
	usb_descriptors   string
	ipp_attributes    string
	escl_capabilities string
	cleanup           bool
}

func getVars(s *testing.State) deviceVars {
	const (
		defaultDescriptors  = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
		defaultAttributes   = "/usr/local/etc/virtual-usb-printer/ipp_attributes.json"
		defaultCapabilities = "/usr/local/etc/virtual-usb-printer/escl_capabilities.json"
	)

	usb_descriptors, ok := s.Var("descriptors")
	if !ok {
		usb_descriptors = defaultDescriptors
	}

	ipp_attributes, ok := s.Var("attributes")
	if !ok {
		ipp_attributes = defaultAttributes
	}

	escl_capabilities, ok := s.Var("escl_caps")
	if !ok {
		escl_capabilities = defaultCapabilities
	}

	return deviceVars{
		usb_descriptors:   usb_descriptors,
		ipp_attributes:    ipp_attributes,
		escl_capabilities: escl_capabilities,
	}
}

func StartVirtualScanner(ctx context.Context, s *testing.State) {
	vars := getVars(s)

	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	if err := usbprinter.InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}
	defer func(ctx context.Context) {
		if vars.cleanup {
			if err := usbprinter.RemoveModules(ctx); err != nil {
				s.Error("Failed to remove kernel modules: ", err)
			}
		}
	}(cleanupCtx)

	devInfo, err := usbprinter.LoadPrinterIDs(vars.usb_descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", vars.usb_descriptors, err)
	}

	if err := cups.RestartPrintingSystem(ctx, devInfo); err != nil {
		s.Fatal("Failed to restart printing system: ", err)
	}

	printer, err := usbprinter.StartScanner(ctx, devInfo, vars.usb_descriptors, vars.ipp_attributes, vars.escl_capabilities, "")
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func() {
		if vars.cleanup && printer != nil {
			usbprinter.StopPrinter(cleanupCtx, printer, devInfo)
		}
	}()
	if err := cups.EnsurePrinterIdle(ctx, devInfo); err != nil {
		s.Fatal("Failed to wait for CUPS configuration: ", err)
	}

	deviceName := fmt.Sprintf("ippusb:escl:TestScanner:%s_%s/eSCL", devInfo.VID, devInfo.PID)
	s.Log("Scanner is available at %s", deviceName)
}
