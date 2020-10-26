// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanner

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/process"

	lpb "chromiumos/system_api/lorgnette_proto"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/scanner/lorgnette"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

type params struct {
	Network bool // Set up as a network device instead of local.
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScanESCLIPP,
		Desc:         "Tests eSCL scanning via an ipp-over-usb tunnel",
		Contacts:     []string{"fletcherw@chromium.org", "project-bolton@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"virtual_usb_printer"},
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

func killIPPUSBBridge(ctx context.Context, devInfo usbprinter.DevInfo) error {
	ps, err := process.Processes()
	if err != nil {
		return err
	}

	for _, p := range ps {
		if name, err := p.Name(); err != nil || name != "ippusb_bridge" {
			continue
		}

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
		if err := os.Remove(fmt.Sprintf("/run/ippusb/%s-%s_keep_alive.sock", devInfo.VID, devInfo.PID)); err != nil && !os.IsNotExist(err) {
			return errors.Wrap(err, "failed to remove ippusb_bridge keepalive socket")
		}
	}
	return nil
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

	printer, err := usbprinter.StartScanner(ctx, devInfo, descriptors, attributes, esclCapabilities, s.DataPath(sourceImage))
	if err != nil {
		s.Fatal("Failed to attach virtual printer: ", err)
	}
	defer func() {
		usbprinter.StopPrinter(cleanupCtx, printer, devInfo)
	}()

	var deviceName string
	if testOpt.Network {
		// To simulate a network scanner, we start up ippusb_bridge manually and have it listen on localhost:60000.
		// For USB scanners, lorgnette will automatically contact ippusb_manager to set up ippusb_bridge properly.
		bus, device, err := findScanner(ctx, devInfo)
		if err != nil {
			s.Fatal("Failed to find scanner bus device: ", err)
		}

		s.Log("Setting up ipp-usb connection")
		ippusbBridge := testexec.CommandContext(ctx, "ippusb_bridge", "--bus-device", fmt.Sprintf("%s:%s", bus, device))

		if err := ippusbBridge.Start(); err != nil {
			s.Fatal("Failed to connect to printer with ippusb_bridge: ", err)
		}
		defer func() {
			ippusbBridge.Kill()
			ippusbBridge.Wait()
		}()

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

		deviceName = fmt.Sprintf("airscan:escl:TestScanner:http://localhost:%d/eSCL", port)
	} else {
		deviceName = fmt.Sprintf("ippusb:escl:TestScanner:%s_%s/eSCL", devInfo.VID, devInfo.PID)

		// In the USB case, ippusb_bridge is started indirectly by lorgnette, so we don't
		// have a process to kill directly.  Instead, search the process tree.
		defer killIPPUSBBridge(cleanupCtx, devInfo)
	}

	tmpDir, err := ioutil.TempDir("", "tast.scanner.ScanEsclIPP.")
	if err != nil {
		s.Fatal("Failed to create temporary directory: ", err)
	}
	defer os.RemoveAll(tmpDir)

	scanPath := filepath.Join(tmpDir, "scanned.png")
	scanFile, err := os.Create(scanPath)
	if err != nil {
		s.Fatal("Failed to open scan output file: ", err)
	}

	startScanRequest := &lpb.StartScanRequest{
		DeviceName: deviceName,
		Settings: &lpb.ScanSettings{
			Resolution: 300,
			SourceName: "Flatbed",
			ColorMode:  lpb.ColorMode_MODE_COLOR,
		},
	}

	l, err := lorgnette.New(ctx)
	if err != nil {
		s.Fatal("Failed to connect to lorgnette: ", err)
	}

	s.Log("Starting scan")
	startScanResponse, err := l.StartScan(ctx, startScanRequest)
	if err != nil {
		s.Fatal("Failed to call StartScan: ", err)
	}
	// Lorgnette was started automatically when we called StartScan, make sure to
	// close it when we exit.
	defer lorgnette.StopService(cleanupCtx)

	switch startScanResponse.State {
	case lpb.ScanState_SCAN_STATE_IN_PROGRESS:
		// Do nothing.
	case lpb.ScanState_SCAN_STATE_FAILED:
		s.Fatal("Failed to start scan: ", startScanResponse.FailureReason)
	default:
		s.Fatal("Unexpected ScanState: ", startScanResponse.State.String())
	}

	getNextImageRequest := &lpb.GetNextImageRequest{
		ScanUuid: startScanResponse.ScanUuid,
	}

	s.Log("Getting next image")
	getNextImageResponse, err := l.GetNextImage(ctx, getNextImageRequest, scanFile.Fd())
	if err != nil {
		s.Fatal("Failed to call GetNextImage: ", err)
	}

	if !getNextImageResponse.Success {
		s.Fatal("Failed to get next image: ", getNextImageResponse.FailureReason)
	}

	s.Log("Waiting for completion signal")
	if err = l.WaitForScanCompletion(ctx, startScanResponse.ScanUuid); err != nil {
		s.Fatal("Failed to wait for scan completion: ", err)
	}

	s.Log("Comparing scanned file to golden image")
	diff := testexec.CommandContext(ctx, "perceptualdiff", "-verbose", "-threshold", "1", scanPath, s.DataPath(goldenImage))
	if err := diff.Run(); err != nil {
		s.Error("Scanned file differed from golden image: ", err)
		diff.DumpLog(ctx)
	}
}
