// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanner

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	lpb "chromiumos/system_api/lorgnette_proto"
	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/scanner/lorgnette"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/printing/cups"
	"chromiumos/tast/local/printing/ippusbbridge"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/testing"
)

type params struct {
	Network bool // Set up as a network device instead of local.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:     ScanESCLIPP,
		Desc:     "Tests eSCL scanning via an ipp-over-usb tunnel",
		Contacts: []string{"bmgordon@chromium.org", "project-bolton@google.com"},
		Attr: []string{
			"group:mainline",
			"group:paper-io",
			"paper-io_scanning",
		},
		SoftwareDeps: []string{"virtual_usb_printer", "cups", "chrome"},
		Pre:          chrome.LoggedIn(),
		Data:         []string{sourceImage, goldenImage},
		Params: []testing.Param{{
			Name: "usb",
			Val: &params{
				Network: false,
			},
		}, {
			Name: "network",
			Val: &params{
				Network: true,
			},
		}},
	})
}

const (
	sourceImage = "scan_escl_ipp_source.jpg"
	goldenImage = "scan_escl_ipp_golden.png"
)

// findScanner runs lsusb in order to find the Bus and Device number for the USB
// device with the VID and PID given in devInfo.
func findScanner(ctx context.Context, devInfo usbprinter.DevInfo) (bus, device string, err error) {
	b, err := testexec.CommandContext(ctx, "lsusb", "-d", fmt.Sprintf("%s:%s", devInfo.VID, devInfo.PID)).Output(testexec.DumpLogOnError)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to run lsusb")
	}

	out := string(b)
	colonIndex := strings.Index(out, ":")
	if colonIndex == -1 {
		return "", "", errors.Wrap(err, "failed to find ':' in lsusb output")
	}

	tokens := strings.Split(out[:colonIndex], " ")
	if len(tokens) != 4 || tokens[0] != "Bus" || tokens[2] != "Device" {
		return "", "", errors.Errorf("failed to parse output as Bus [bus-id] Device [device-id]: %s", out)
	}

	return tokens[1], tokens[3], nil
}

func ScanESCLIPP(ctx context.Context, s *testing.State) {
	const (
		descriptors      = "/usr/local/etc/virtual-usb-printer/ippusb_printer.json"
		attributes       = "/usr/local/etc/virtual-usb-printer/ipp_attributes.json"
		esclCapabilities = "/usr/local/etc/virtual-usb-printer/escl_capabilities.json"
	)

	testOpt := s.Param().(*params)

	// Use cleanupCtx for any deferred cleanups in case of timeouts or
	// cancellations on the shortened context.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

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

	if err := cups.RestartPrintingSystem(ctx, devInfo); err != nil {
		s.Fatal("Failed to restart printing system: ", err)
	}

	printer, err := usbprinter.StartScanner(ctx, devInfo, descriptors, attributes, esclCapabilities, s.DataPath(sourceImage), "")
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func() {
		if printer != nil {
			usbprinter.StopPrinter(cleanupCtx, printer, devInfo)
		}
	}()
	if err := cups.EnsurePrinterIdle(ctx, devInfo); err != nil {
		s.Fatal("Failed to wait for CUPS configuration: ", err)
	}

	var deviceName string
	if testOpt.Network {
		// To simulate a network scanner, we start up ippusb_bridge manually and have it listen on localhost:60000.
		// Normally udev automatically starts ippusb_bridge when a USB printer is plugged in, so we have to manually
		// stop the job first.
		bus, device, err := findScanner(ctx, devInfo)
		if err != nil {
			s.Fatal("Failed to find scanner bus device: ", err)
		}

		s.Log("Stopping ippusb-bridge job")
		cmd := testexec.CommandContext(ctx, "initctl", "stop", "ippusb-bridge", fmt.Sprintf("BUS=%03s", bus), fmt.Sprintf("DEV=%03s", device))
		if err := cmd.Run(); err != nil {
			s.Fatalf("Failed to stop ippusb-bridge instance for BUS=%03s DEV=%03s: %v", bus, device, err)
		}

		s.Log("Setting up ipp-usb connection")
		ippusbBridge := testexec.CommandContext(ctx, "ippusb_bridge", "--bus-device", fmt.Sprintf("%s:%s", bus, device), "--keep-alive", ippusbbridge.KeepAlivePath(devInfo))

		if err := ippusbBridge.Start(); err != nil {
			s.Fatal("Failed to connect to printer with ippusb_bridge: ", err)
		}
		defer ippusbbridge.Kill(cleanupCtx, devInfo)

		// Defined in src/platform2/ippusb_bridge/src/main.rs
		const port = 60000

		// Wait for ippusb_bridge to start up.
		if err := testing.Poll(ctx, func(ctx context.Context) error {
			d := &net.Dialer{}
			_, err := d.DialContext(ctx, "tcp", fmt.Sprintf("localhost:%d", port))
			return err
		}, &testing.PollOptions{
			Timeout:  10 * time.Second,
			Interval: 1 * time.Second,
		}); err != nil {
			s.Fatal("Failed to wait for ippusb_bridge to start: ", err)
		}

		// Even after ippusb_bridge is listening, it may take a few seconds to finish
		// probing USB devices before it responds.  In order to avoid lorgnette timing out,
		// do an initial query to make sure traffic is passing through the tunnel.
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/eSCL/ScannerCapabilities", port))
		if err != nil {
			s.Fatal("Failed to send query to ippusb_bridge: ", err)
		}
		_, err = ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			s.Fatal("Failed to read response from ippusb_bridge: ", err)
		}

		deviceName = fmt.Sprintf("airscan:escl:TestScanner:http://localhost:%d/eSCL", port)
	} else {
		deviceName = fmt.Sprintf("ippusb:escl:TestScanner:%s_%s/eSCL", devInfo.VID, devInfo.PID)

		// In the USB case, ippusb_bridge is started indirectly by lorgnette, so we don't
		// have a process to kill directly.  Instead, search the process tree.
		defer ippusbbridge.Kill(cleanupCtx, devInfo)
	}

	tmpDir, err := ioutil.TempDir("", "tast.scanner.ScanEsclIPP.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(tmpDir)

	startScanRequest := &lpb.StartScanRequest{
		DeviceName: deviceName,
		Settings: &lpb.ScanSettings{
			Resolution: 300,
			SourceName: "Flatbed",
			ColorMode:  lpb.ColorMode_MODE_COLOR,
		},
	}

	scanPath := lorgnette.RunScan(ctx, s, startScanRequest, tmpDir)

	s.Log("Comparing scanned file to golden image")
	diff := testexec.CommandContext(ctx, "perceptualdiff", "-verbose", "-threshold", "1", scanPath, s.DataPath(goldenImage))
	if err := diff.Run(); err != nil {
		s.Error("Scanned file differed from golden image: ", err)
		diff.DumpLog(ctx)
	}

	// Intentionally stop the printer early to trigger shutdown in ippusb_bridge.
	// Without this, cleanup may have to wait for other processes to finish using
	// the printer (e.g. CUPS background probing).
	usbprinter.StopPrinter(cleanupCtx, printer, devInfo)
	printer = nil
}
