// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         UsbAttachToArcvm,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Validity attaching virtual usb device to arcvm",
		Contacts:     []string{"lgcheng@google.com", "arc-eng@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm"},
		}},
		Timeout: chrome.GAIALoginTimeout + arc.BootTimeout + 1*time.Minute,
		VarDeps: []string{"ui.gaiaPoolDefault"},
	})
}

const (
	usbVID          = "dddd"
	usbPID          = "ffff"
	usbManufacturer = "Tast"
	usbProduct      = "VirtualTestUSBDrive"
	usbSerialNumber = "12345"
)

func UsbAttachToArcvm(ctx context.Context, s *testing.State) {
	// Enable the feature flag in case it's disabled by default
	args := append(arc.DisableSyncFlags(), "--enable-features=UsbDeviceDefaultAttachToArcVm")
	// Setup Chrome.
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(s.RequiredVar("ui.gaiaPoolDefault")),
		chrome.ARCSupported(),
		chrome.ExtraArgs(args...))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create test API connection: ", err)
	}

	// Optin to play store. This step is needed to access manage usb sub page,
	// which is a subpage of Google Play Store settings page.
	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		s.Fatal("Failed to optin to Play Store and Close: ", err)
	}

	// Setup ARC.
	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		s.Fatal("Failed to start ARC: ", err)
	}
	defer a.Close(ctx)

	// Verify virtual USB device not in host nor ARCVM
	if err := checkUsbDeviceStatus(ctx, false, false); err != nil {
		s.Fatal("Wrong inital USB state: ", err)
	}

	// Create the virtual USB device
	if err := setupVirtualUsbDevice(ctx); err != nil {
		s.Fatal("Fail to setup virtual USB device: ", err)
	}
	defer cleanupVirtualUsbDevice(ctx)

	// Verify deice shown in host but not in ARCVM
	if err := checkUsbDeviceStatus(ctx, true, false); err != nil {
		s.Fatal("Wrong USB state after creating virtual USB device: ", err)
	}

	// Attach the virtual USB device and verify it shown in host and in ARCVM
	if err := attachUsbDeviceToARCVM(ctx, cr, tconn); err != nil {
		s.Fatal("Fail to Attach virtual USB device to vm: ", err)
	}
	//Verify the virtual device shown in host and in ARCVM
	if err := checkUsbDeviceStatus(ctx, true, true); err != nil {
		s.Fatal("Wrong USB state after creating virtual USB device: ", err)
	}
}

// setupVirtualUsbDevice with test config.
func setupVirtualUsbDevice(ctx context.Context) error {
	if err := testexec.CommandContext(ctx, "modprobe",
		"dummy_hcd").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "Fail to load dummy_hcd module")
	}

	if err := testexec.CommandContext(ctx, "dd", "bs=1024", "count=64", "if=/dev/zero",
		"of=/tmp/backing_file").Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "Fail to create temporary backing_file")
	}

	if err := testexec.CommandContext(ctx, "modprobe", "g_mass_storage",
		"file=/tmp/backing_file", "idVendor=0x"+usbVID, "idProduct=0x"+usbPID,
		"iManufacturer="+usbManufacturer, "iProduct="+usbProduct,
		"iSerialNumber="+usbSerialNumber).Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "Fail to create virtual USB storge")
	}

	return nil
}

// cleanupVirtualUsbDevice after test finished.
// No op if fail to create virtual USB device at setup.
func cleanupVirtualUsbDevice(ctx context.Context) {
	testexec.CommandContext(ctx, "modprobe", "g_mass_storage", "-r").Run()
}

// attachUsbDeviceToARCVM through Manage USB settings page.
func attachUsbDeviceToARCVM(ctx context.Context, cr *chrome.Chrome, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)
	playStoreButton := nodewith.Name("Google Play Store").Role(role.Button)
	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, cr, "apps",
		ui.Exists(playStoreButton)); err != nil {
		return errors.Wrap(err, "failed to launch apps settings page")
	}

	return uiauto.Combine("Manage USB",
		ui.FocusAndWait(playStoreButton),
		ui.LeftClick(playStoreButton),
		ui.LeftClick(nodewith.Name("Manage USB devices").Role(role.Link)),
		ui.LeftClick(nodewith.Name(usbProduct).Role(role.ToggleButton)),
	)(ctx)
}

// checkUsbDeviceStatus which presnented by string |VID:PID| exists in host
// and/or vm.
// |host| device is expected to be shown in host os.
// |vm| device is expected to be shown in vm.
func checkUsbDeviceStatus(ctx context.Context, host, vm bool) error {
	return testing.Poll(ctx, func(context.Context) error {
		// Check whether USB device presented as |VID:PID| exists in host
		// If the specified device is not found, a non-zero exit code is returned
		// by lsusb and err will not be nil
		// |err == nil| indicates |lsusb -d XXXX:XXXX| finds the expected devices.
		if err := testexec.CommandContext(ctx, "lsusb", "-d",
			fmt.Sprintf("%s:%s", usbVID,
				usbPID)).Run(testexec.DumpLogOnError); (err == nil) != host {
			return errors.Wrap(err, "unexpected usb status in host os")
		}

		// Check whether USB device presented as |VID:PID| exists in ARCVM
		// lsusb inside arcvm does not support addition arguments.
		// e.g lsusb -d XXXX:XXXX returns same result as lsusb
		// |err == nil| indicates |grep| finds the expected device.
		if err := testexec.CommandContext(ctx, "/usr/sbin/android-sh", "-c",
			"lsusb | grep "+fmt.Sprintf("%s:%s", usbVID,
				usbPID)).Run(testexec.DumpLogOnError); (err == nil) != vm {
			return errors.Wrap(err, "unexpected usb status in vm")
		}
		return nil
	}, &testing.PollOptions{Timeout: time.Second})
}
