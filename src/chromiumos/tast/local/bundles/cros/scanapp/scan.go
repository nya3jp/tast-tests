// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanapp

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/faillog"
	"chromiumos/tast/local/chrome/ui/scanapp"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Scan,
		Desc: "Tests that the Scan app can be used to perform a scan",
		Contacts: []string{
			"jschettler@chromium.org",
			"cros-peripherals@google.com",
			"project-bolton@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome", "virtual_usb_printer"},
		Data:         []string{sourceImage},
	})
}

const (
	sourceImage = "scan_source.jpg"
)

func keepAlivePath(devInfo usbprinter.DevInfo) string {
	return fmt.Sprintf("/run/ippusb/%s-%s_keep_alive.sock", devInfo.VID, devInfo.PID)
}

func killIPPUSBBridge(ctx context.Context, devInfo usbprinter.DevInfo) error {
	ps, err := process.Processes()
	if err != nil {
		return err
	}

	for _, p := range ps {
		if name, err := p.Name(); err != nil || name != "ippusb_bridge" {
			continue
		}

		testing.ContextLog(ctx, "Killing ippusb_bridge with pid ", p.Pid)
		if err := syscall.Kill(int(p.Pid), syscall.SIGINT); err != nil && err != syscall.ESRCH {
			return errors.Wrap(err, "failed to kill ippusb_bridge")
		}

		// Wait for the process to exit so that its sockets can be removed.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			// We need a fresh process.Process since it caches attributes.
			// TODO(crbug.com/1131511): Clean up error handling here when gpsutil has been upreved.
			if _, err := process.NewProcess(p.Pid); err == nil {
				return errors.Errorf("pid %d is still running", p.Pid)
			}
			return nil
		}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
			return errors.Wrap(err, "failed to wait for ippusb_bridge to exit")
		}
		if err := os.Remove(fmt.Sprintf("/run/ippusb/%s-%s.sock", devInfo.VID, devInfo.PID)); err != nil && !os.IsNotExist(err) {
			return errors.Wrap(err, "failed to remove ippusb_bridge socket")
		}
		if err := os.Remove(keepAlivePath(devInfo)); err != nil && !os.IsNotExist(err) {
			return errors.Wrap(err, "failed to remove ippusb_bridge keepalive socket")
		}
	}
	return nil
}

func Scan(ctx context.Context, s *testing.State) {
	const (
		descriptors      = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
		attributes       = "/usr/local/etc/virtual-usb-printer/ipp_attributes.json"
		esclCapabilities = "/usr/local/etc/virtual-usb-printer/escl_capabilities.json"
	)

	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Create a Chrome instance with the Scan app enabled.
	cr, err := chrome.New(ctx, chrome.EnableFeatures("ScanningUI"))
	if err != nil {
		s.Fatal("Failed to create Chrome instance: ", err)
	}
	defer cr.Close(cleanupCtx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect Test API: ", err)
	}
	defer faillog.DumpUITreeOnError(cleanupCtx, s.OutDir(), s.HasError, tconn)

	// Set up the virtual USB printer.
	if err := usbprinter.InstallModules(ctx); err != nil {
		s.Fatal("Failed to install kernel modules: ", err)
	}
	defer func(ctx context.Context) {
		if err := usbprinter.RemoveModules(ctx); err != nil {
			s.Error("Failed to remove kernel modules: ", err)
		}
	}(cleanupCtx)

	devInfo, err := usbprinter.LoadPrinterIDs(descriptors)
	if err != nil {
		s.Fatalf("Failed to load printer IDs from %v: %v", descriptors, err)
	}

	printer, err := usbprinter.StartScanner(ctx, devInfo, descriptors, attributes, esclCapabilities, s.DataPath(sourceImage))
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func() {
		if printer != nil {
			usbprinter.StopPrinter(cleanupCtx, printer, devInfo)
		}
	}()

	// In the USB case, ippusb_bridge is started indirectly by lorgnette, so we
	// don't have a process to kill directly. Instead, search the process tree.
	defer killIPPUSBBridge(cleanupCtx, devInfo)

	// Launch the Scan app and perform a scan.
	app, err := scanapp.Launch(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to launch app: ", err)
	}
	defer app.Release(ctx)

	if err := app.Scan(ctx); err != nil {
		s.Fatal("Failed to perform scan: ", err)
	}

	if err := testing.Sleep(ctx, 15*time.Second); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	// Intentionally stop the printer early to trigger shutdown in ippusb_bridge.
	// Without this, cleanup may have to wait for other processes to finish using
	// the printer (e.g. CUPS background probing).
	usbprinter.StopPrinter(cleanupCtx, printer, devInfo)
	printer = nil
}
