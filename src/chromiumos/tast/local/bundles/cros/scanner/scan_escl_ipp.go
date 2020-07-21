// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScanESCLIPP,
		Desc:         "Tests eSCL scanning via an ipp-over-usb tunnel",
		Contacts:     []string{"fletcherw@chromium.org", "project-bolton@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"virtual_usb_printer"},
		Data:         []string{sourceImage, goldenImage},
	})
}

const (
	sourceImage = "scan_escl_ipp_source.jpg"
	goldenImage = "scan_escl_ipp_golden.png"
)

// getIPPUSBBridgePort scans r, which contains output from ippusb_bridge, for the port
// that it connected to, and returns it.
func getIPPUSBBridgePort(r io.Reader) (int, error) {
	reader := bufio.NewReader(r)
	token, err := reader.ReadString('|')
	if err != nil {
		return 0, errors.Wrap(err, "failed to read from ippusb_bridge pipe")
	}
	// Trim off last character since it's the '|' delimiter.
	port, err := strconv.Atoi(token[:len(token)-1])
	if err != nil {
		return 0, errors.Wrap(err, "failed to parse port as integer")
	}
	return port, nil
}

func findScanner(ctx context.Context, devInfo usbprinter.DevInfo) (string, string, error) {
	lsusb := testexec.CommandContext(ctx, "lsusb", "-d", fmt.Sprintf("%s:%s", devInfo.VID, devInfo.PID))
	pipe, err := lsusb.StdoutPipe()
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get lsusb output pipe")
	}

	if err := lsusb.Start(); err != nil {
		return "", "", errors.Wrap(err, "failed to run lsusb")
	}

	reader := bufio.NewReader(pipe)
	output, err := reader.ReadString(':')
	if err != nil {
		return "", "", errors.Wrap(err, "failed to read from lsusb pipe")
	}
	// Trim off trailing ':'.
	output = output[:len(output)-1]

	tokens := strings.Split(output, " ")
	if len(tokens) != 4 {
		return "", "", errors.Errorf("failed to parse output as Bus [bus-id] Device [device-id]: %s", output)
	}

	lsusb.Wait()

	return tokens[1], tokens[3], nil
}

func ScanESCLIPP(ctx context.Context, s *testing.State) {
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
		printer.Kill()
		printer.Wait()
	}()

	bus, device, err := findScanner(ctx, devInfo)
	if err != nil {
		s.Fatal("Failed to find scanner bus device: ", err)
	}

	s.Log("Setting up ipp-usb connection")
	ippusbBridge := testexec.CommandContext(ctx, "ippusb_bridge", "--bus-device", fmt.Sprintf("%s:%s", bus, device))
	//pipe, err := ippusbBridge.StdoutPipe()
	//if err != nil {
	//	s.Fatal("Failed to get ippusb_bridge stdout pipe: ", err)
	//}

	if err := ippusbBridge.Start(); err != nil {
		s.Fatal("Failed to connect to printer with ippusb_bridge: ", err)
	}
	defer func() {
		ippusbBridge.Kill()
		ippusbBridge.Wait()
	}()

	//port, err := getIPPUSBBridgePort(pipe)
	//if err != nil {
	//	s.Fatal("Failed to get ippusb_bridge port: ", err)
	//}
	port := 60000
	s.Log("Connected to virtual printer with ippusb_bridge")

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

	scannerName := fmt.Sprintf("airscan:escl:TestScanner:http://localhost:%d/eSCL", port)
	fileDescriptor := dbus.UnixFD(scanFile.Fd())
	scanProperties := map[string]dbus.Variant{
		"Resolution": dbus.MakeVariant(uint32(300)),
		"Mode":       dbus.MakeVariant("Color"),
	}

	const (
		dbusName      = "org.chromium.lorgnette"
		dbusPath      = "/org/chromium/lorgnette/Manager"
		dbusInterface = "org.chromium.lorgnette.Manager"
	)

	conn, err := dbusutil.SystemBus()
	if err != nil {
		s.Fatal("Failed to connect to system bus: ", err)
	}

	s.Log("Requesting Lorgnette to ScanImage")
	obj := conn.Object(dbusName, dbus.ObjectPath(dbusPath))
	if err := obj.CallWithContext(ctx, dbusInterface+".ScanImage", 0, scannerName, fileDescriptor, scanProperties).Err; err != nil {
		s.Fatal("Failed to ScanImage: ", err)
	}

	s.Log("Comparing scanned file to golden image")
	diff := testexec.CommandContext(ctx, "perceptualdiff", "-verbose", "-threshold", "1", scanPath, s.DataPath(goldenImage))
	if err := diff.Run(); err != nil {
		s.Error("Scanned file differed from golden image: ", err)
		diff.DumpLog(ctx)
	}
}
