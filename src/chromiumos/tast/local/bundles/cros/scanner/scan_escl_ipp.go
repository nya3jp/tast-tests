// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package scanner

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/godbus/dbus"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/dbusutil"
	"chromiumos/tast/local/printing/usbprinter"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const (
	sourceImage = "scan_escl_ipp_source.jpg"
	goldenImage = "scan_escl_ipp_golden.png"
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

	// FIXME: Have it pick an arbitrary port once direct device spec has been
	// merged.
	ippusbxd := testexec.CommandContext(ctx, "ippusbxd", "-v", devInfo.VID, "-m", devInfo.PID, "-p", "3333")
	if err := ippusbxd.Start(); err != nil {
		s.Fatal("Failed to connect to printer with ippusbxd: ", err)
	}
	defer func() {
		ippusbxd.Kill()
		ippusbxd.Wait()
	}()

	s.Log("Connected to virtual printer with ippusbxd")

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

	// FIXME: Use direct device specification once feature has been merged to
	// airscan.
	scannerName := "airscan:e0:AirscanTest"
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
